// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter_test

import (
	"encoding/json"
	"errors"
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("dashboard url", func() {
	const externalBinPath = "/thing"

	var (
		a          *serviceadapter.Client
		cmdRunner  *fakes.FakeCommandRunner
		logs       *gbytes.Buffer
		logger     *log.Logger
		instanceID string
		plan       sdk.Plan
		manifest   []byte

		actualDashboardUrl string
		actualError        error
	)

	BeforeEach(func() {
		logs = gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(GinkgoWriter, logs), "[unit-tests] ", log.LstdFlags)
		cmdRunner = new(fakes.FakeCommandRunner)
		a = &serviceadapter.Client{
			CommandRunner:   cmdRunner,
			ExternalBinPath: externalBinPath,
		}
		plan = sdk.Plan{
			Properties: sdk.Properties{
				"foo": "bar",
				"baz": map[string]interface{}{
					"qux": "quux",
				},
			},
		}

		manifest = []byte("a manifest")

		cmdRunner.RunReturns([]byte(""), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)

		instanceID = "the-instance-id"
		manifest = []byte("a-manifest")

		dashboardUrlJSON := []byte(`{ "dashboard_url": "https://someurl.com"}`)

		cmdRunner.RunReturns(dashboardUrlJSON, []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
		cmdRunner.RunWithInputParamsReturns(dashboardUrlJSON, []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
	})

	JustBeforeEach(func() {
		actualDashboardUrl, actualError = a.GenerateDashboardUrl(instanceID, plan, manifest, logger)
	})
	Context("when stdin is not set", func() {

		It("invokes external dashboard url generator with serialised params", func() {
			Expect(cmdRunner.RunCallCount()).To(Equal(1))
			planJson, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())
			argsPassed := cmdRunner.RunArgsForCall(0)
			Expect(argsPassed).To(ConsistOf(externalBinPath, "dashboard-url", instanceID, string(planJson), string(manifest)))
		})

		It("returns a dashboard url", func() {
			Expect(actualDashboardUrl).To(Equal("https://someurl.com"))
		})

		It("has no error", func() {
			Expect(actualError).NotTo(HaveOccurred())
		})

		It("logs adapter stderr", func() {
			Expect(logs).To(gbytes.Say("I'm stderr"))
		})

		Context("when plan properties are formatted as map[interface][interface]", func() {
			BeforeEach(func() {
				plan = sdk.Plan{
					Properties: sdk.Properties{
						"foo": "bar",
						"baz": map[interface{}]interface{}{
							"qux": "quux",
						},
					},
				}
			})

			It("converts plan properties to be json serializable", func() {
				Expect(actualError).NotTo(HaveOccurred())
				Expect(cmdRunner.RunCallCount()).To(Equal(1))
				argsPassed := cmdRunner.RunArgsForCall(0)

				convertedPlan := sdk.Plan{
					Properties: sdk.Properties{
						"foo": "bar",
						"baz": map[string]interface{}{
							"qux": "quux",
						},
					},
				}

				convertedPlanJson, err := json.Marshal(convertedPlan)
				Expect(err).NotTo(HaveOccurred())
				Expect(argsPassed).To(ConsistOf(externalBinPath, "dashboard-url", instanceID, string(convertedPlanJson), string(manifest)))
			})
		})

		Context("adapter did not implement the dashboard url subcommand", func() {
			BeforeEach(func() {
				cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(actualError.Error()).NotTo(ContainSubstring("stdout"))
				Expect(actualError.Error()).NotTo(ContainSubstring("stderr"))
			})

			It("logs the operator error message", func() {
				Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})

		Context("when the external service adapter fails, without an exit code", func() {
			var err = errors.New("oops")

			BeforeEach(func() {
				cmdRunner.RunReturns(nil, nil, nil, err)
			})

			It("returns an error", func() {
				Expect(actualError).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is a operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}))
					Expect(actualError.Error()).To(Equal("I'm stdout"))
				})

				It("logs an message to the operator", func() {
					Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'"))
				})
			})
		})

		Context("cannot deserialise response", func() {
			BeforeEach(func() {
				cmdRunner.RunReturns([]byte("invalid json"), []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
			})

			It("returns an error", func() {
				Expect(actualError).To(HaveOccurred())
			})
		})
	})

	Context("when stdin is set", func() {
		BeforeEach(func() {
			a.UsingStdin = true
		})

		It("generates the dashboard url", func() {
			planJson, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())

			By("invoking the handler")
			Expect(cmdRunner.RunCallCount()).To(Equal(0))
			Expect(cmdRunner.RunWithInputParamsCallCount()).To(Equal(1))
			inputParams, argsPassed := cmdRunner.RunWithInputParamsArgsForCall(0)
			Expect(inputParams.(sdk.InputParams)).To(Equal(sdk.InputParams{
				DashboardUrl: sdk.DashboardUrlJSONParams{
					InstanceId: instanceID, Plan: string(planJson), Manifest: string(manifest),
				},
			}))
			Expect(argsPassed).To(ConsistOf(externalBinPath, "dashboard-url"))

			By("returns a dashboard url")
			Expect(actualDashboardUrl).To(Equal("https://someurl.com"))

			By("not returning an error")
			Expect(actualError).NotTo(HaveOccurred())

			By("logging the adapter stderr")
			Expect(logs).To(gbytes.Say("I'm stderr"))
		})

		Context("when plan properties are formatted as map[interface][interface]", func() {
			BeforeEach(func() {
				plan = sdk.Plan{
					Properties: sdk.Properties{
						"foo": "bar",
						"baz": map[interface{}]interface{}{
							"qux": "quux",
						},
					},
				}
			})

			It("converts plan properties to be json serializable", func() {
				Expect(actualError).NotTo(HaveOccurred())
				Expect(cmdRunner.RunWithInputParamsCallCount()).To(Equal(1))
				inputParams, argsPassed := cmdRunner.RunWithInputParamsArgsForCall(0)

				convertedPlan := sdk.Plan{
					Properties: sdk.Properties{
						"foo": "bar",
						"baz": map[string]interface{}{
							"qux": "quux",
						},
					},
				}

				convertedPlanJson, err := json.Marshal(convertedPlan)
				Expect(err).NotTo(HaveOccurred())
				Expect(inputParams.(sdk.InputParams)).To(Equal(sdk.InputParams{
					DashboardUrl: sdk.DashboardUrlJSONParams{
						InstanceId: instanceID, Plan: string(convertedPlanJson), Manifest: string(manifest),
					},
				}))
				Expect(argsPassed).To(ConsistOf(externalBinPath, "dashboard-url"))
			})
		})

		Context("adapter did not implement the dashboard url subcommand", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(actualError.Error()).NotTo(ContainSubstring("stdout"))
				Expect(actualError.Error()).NotTo(ContainSubstring("stderr"))
			})

			It("logs the operator error message", func() {
				Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})

		Context("when the external service adapter fails, without an exit code", func() {
			var err = errors.New("oops")

			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns(nil, nil, nil, err)
			})

			It("returns an error", func() {
				Expect(actualError).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is a operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunWithInputParamsReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}))
					Expect(actualError.Error()).To(Equal("I'm stdout"))
				})

				It("logs an message to the operator", func() {
					Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'"))
				})
			})
		})

		Context("cannot deserialise response", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("invalid json"), []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
			})

			It("returns an error", func() {
				Expect(actualError).To(HaveOccurred())
			})
		})
	})
})
