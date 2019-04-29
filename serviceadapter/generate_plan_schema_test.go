// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serviceadapter_test

import (
	"encoding/json"
	"errors"
	"io"
	"log"

	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("GeneratePlanSchema", func() {
	const externalBinPath = "/service-adapter"

	var (
		a         *serviceadapter.Client
		cmdRunner *fakes.FakeCommandRunner
		logs      *gbytes.Buffer
		logger    *log.Logger
		plan      sdk.Plan

		expectedPlanSchemas domain.ServiceSchemas
		actualPlanSchemas   domain.ServiceSchemas

		actualError error
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

		expectedPlanSchemas = getPlanSchema()

		buf := gbytes.NewBuffer()
		Expect(json.NewEncoder(buf).Encode(expectedPlanSchemas)).To(Succeed())
		cmdRunner.RunReturns(buf.Contents(), []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)

	})

	JustBeforeEach(func() {
		actualPlanSchemas, actualError = a.GeneratePlanSchema(plan, logger)
	})

	When("UsingStdin is set to false", func() {
		It("invokes external generate plan schema with plan json", func() {
			Expect(actualError).NotTo(HaveOccurred())

			Expect(cmdRunner.RunCallCount()).To(Equal(1))
			planJson, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())
			argsPassed := cmdRunner.RunArgsForCall(0)
			Expect(argsPassed).To(ConsistOf(externalBinPath, "generate-plan-schemas", "--plan-json", string(planJson)))
		})

		It("generates the plan schema", func() {
			Expect(actualError).NotTo(HaveOccurred())

			By("logging whatever the adapter prints to the stderr")
			Expect(logs).To(gbytes.Say("I'm stderr"))

			By("returning the plans schemas")
			Expect(actualPlanSchemas).To(Equal(expectedPlanSchemas))
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
				Expect(argsPassed).To(ConsistOf(externalBinPath, "generate-plan-schemas", "--plan-json", string(convertedPlanJson)))
			})
		})

		Context("adapter did not implement the generate-plan-schemas subcommand", func() {
			BeforeEach(func() {
				cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(actualError.Error()).NotTo(ContainSubstring("stdout"))
				Expect(actualError.Error()).NotTo(ContainSubstring("stderr"))

				By("logs the operator error message")
				Expect(logs).To(gbytes.Say(fmt.Sprintf("external service adapter exited with 10 at %s: stdout: 'stdout', stderr: 'stderr'", externalBinPath)))
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is an operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}))
					Expect(actualError.Error()).To(Equal("I'm stdout"))

					By("logs a message to the operator")
					Expect(logs).To(gbytes.Say(fmt.Sprintf("external service adapter exited with 1 at %s: stdout: 'I'm stdout', stderr: 'I'm stderr'", externalBinPath)))
				})
			})
		})

		Context("when the external service adapter fails, without an exit code", func() {
			var err = errors.New("oops")

			BeforeEach(func() {
				cmdRunner.RunReturns(nil, nil, nil, err)
			})

			It("returns an error", func() {
				Expect(actualError).To(MatchError(fmt.Sprintf("an error occurred running external service adapter at %s: 'oops'. stdout: '', stderr: ''", externalBinPath)))
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

	When("UsingStdin is set to true", func() {
		BeforeEach(func() {
			a.UsingStdin = true

			cmdRunner.RunWithInputParamsReturns([]byte(toJson(expectedPlanSchemas)), []byte("I'm stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
		})

		It("invokes external generate plan schema with plan json", func() {
			Expect(actualError).NotTo(HaveOccurred())

			Expect(cmdRunner.RunCallCount()).To(Equal(0))
			Expect(cmdRunner.RunWithInputParamsCallCount()).To(Equal(1))

			expectedInputParams := sdk.InputParams{
				GeneratePlanSchemas: sdk.GeneratePlanSchemasJSONParams{
					Plan: toJson(plan),
				},
			}
			actualInputParams, argsPassed := cmdRunner.RunWithInputParamsArgsForCall(0)
			Expect(argsPassed).To(ConsistOf(
				externalBinPath,
				"generate-plan-schemas",
			))
			Expect(actualInputParams).To(Equal(expectedInputParams))
		})

		It("generates the plan schema", func() {
			Expect(actualError).NotTo(HaveOccurred())

			By("logging whatever the adapter prints to the stderr")
			Expect(logs).To(gbytes.Say("I'm stderr"))

			By("returning the plans schemas")
			Expect(actualPlanSchemas).To(Equal(expectedPlanSchemas))
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
				actualInputParams, _ := cmdRunner.RunWithInputParamsArgsForCall(0)
				castInputParams, ok := actualInputParams.(sdk.InputParams)
				Expect(ok).To(BeTrue(), "Couldn't cast interface{} back to InputParams")

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
				Expect(castInputParams.GeneratePlanSchemas.Plan).To(Equal(string(convertedPlanJson)))
			})
		})

		Context("adapter did not implement the generate-plan-schemas subcommand", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(actualError).To(HaveOccurred())
				Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(actualError.Error()).NotTo(ContainSubstring("stdout"))
				Expect(actualError.Error()).NotTo(ContainSubstring("stderr"))

				By("logs the operator error message")
				Expect(logs).To(gbytes.Say(fmt.Sprintf("external service adapter exited with 10 at %s: stdout: 'stdout', stderr: 'stderr'", externalBinPath)))
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is an operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunWithInputParamsReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					Expect(actualError).To(BeAssignableToTypeOf(serviceadapter.UnknownFailureError{}))
					Expect(actualError.Error()).To(Equal("I'm stdout"))

					By("logs a message to the operator")
					Expect(logs).To(gbytes.Say(fmt.Sprintf("external service adapter exited with 1 at %s: stdout: 'I'm stdout', stderr: 'I'm stderr'", externalBinPath)))
				})
			})
		})

		Context("when the external service adapter fails, without an exit code", func() {
			var err = errors.New("oops")

			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns(nil, nil, nil, err)
			})

			It("returns an error", func() {
				Expect(actualError).To(MatchError(fmt.Sprintf("an error occurred running external service adapter at %s: 'oops'. stdout: '', stderr: ''", externalBinPath)))
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

func getPlanSchema() domain.ServiceSchemas {
	schemas := domain.Schema{
		Parameters: map[string]interface{}{
			"$schema": "http://json-schema.org/draft-04/schema#",
			"type":    "object",
			"properties": map[string]interface{}{
				"auto_create_topics": map[string]interface{}{
					"description": "Auto create topics",
					"type":        "bool",
					"required":    false,
				},
				"default_replication_factor": map[string]interface{}{
					"description": "Replication factor",
					"type":        "integer",
					"required":    false,
				},
			},
		},
	}
	bindSchema := domain.Schema{
		Parameters: map[string]interface{}{
			"$schema": "http://json-schema.org/draft-04/schema#",
			"type":    "object",
			"properties": map[string]interface{}{
				"topic": map[string]interface{}{
					"description": "The name of the topic",
					"type":        "string",
					"required":    false,
				},
			},
		},
	}
	return domain.ServiceSchemas{
		Instance: domain.ServiceInstanceSchema{
			Create: schemas,
			Update: schemas,
		},
		Binding: domain.ServiceBindingSchema{
			Create: bindSchema,
		},
	}
}
