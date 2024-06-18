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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
)

var _ = Describe("external service adapter", func() {
	const externalBinPath = "/thing"

	var (
		a                  *serviceadapter.Client
		cmdRunner          *fakes.FakeCommandRunner
		logs               *gbytes.Buffer
		logger             *log.Logger
		bindingID          string
		deploymentTopology bosh.BoshVMs
		manifest           []byte
		requestParams      map[string]interface{}
		secrets            map[string]string
		dnsAddresses       map[string]string

		adapterBinding   sdk.Binding
		createBindingErr error
	)

	BeforeEach(func() {
		logs = gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(GinkgoWriter, logs), "[unit-tests] ", log.LstdFlags)
		cmdRunner = new(fakes.FakeCommandRunner)
		a = &serviceadapter.Client{
			CommandRunner:   cmdRunner,
			ExternalBinPath: externalBinPath,
		}
		cmdRunner.RunReturns([]byte(`{
  				"credentials": {
  					"username": "user1",
  					"password": "reallysecret"
  				},
  				"syslog_drain_url": "some-url",
  				"route_service_url": "route-url"
  			}`), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)

		bindingID = "the-binding"
		deploymentTopology = bosh.BoshVMs{"the-deployment": []string{"a-vm"}}
		manifest = []byte("a-manifest")
		requestParams = map[string]interface{}{"foo": "bar"}
		secrets = map[string]string{"admin-password": "pa55w0rd"}
		dnsAddresses = map[string]string{
			"config1": "some.dns.bosh",
			"config2": "some-other.dns.bosh",
		}
	})

	JustBeforeEach(func() {
		adapterBinding, createBindingErr = a.CreateBinding(bindingID, deploymentTopology,
			manifest, requestParams, secrets, dnsAddresses, logger)
	})

	It("invokes external binding creator with serialised params", func() {
		serialisedVMs, err := json.Marshal(deploymentTopology)
		Expect(err).NotTo(HaveOccurred())

		serialisedRequestParams, err := json.Marshal(requestParams)
		Expect(err).NotTo(HaveOccurred())

		Expect(cmdRunner.RunCallCount()).To(Equal(1))
		argsPassed := cmdRunner.RunArgsForCall(0)
		Expect(argsPassed).To(ConsistOf(externalBinPath, "create-binding", bindingID, string(serialisedVMs), string(manifest), string(serialisedRequestParams)))
	})

	Context("when the external adapter succeeds", func() {
		It("returns the service-specific binding output", func() {
			Expect(createBindingErr).ToNot(HaveOccurred())

			Expect(adapterBinding).To(Equal(
				sdk.Binding{
					Credentials:     map[string]interface{}{"username": "user1", "password": "reallysecret"},
					SyslogDrainURL:  "some-url",
					RouteServiceURL: "route-url",
				},
			))
		})
	})

	Context("when the external adapter returns an invalid binding", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns([]byte("invalid json"), []byte("stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
		})

		It("returns the service-specific binding output", func() {
			Expect(createBindingErr).To(
				MatchError(And(
					ContainSubstring("external service adapter returned invalid JSON"),
					ContainSubstring("stdout: 'invalid json', stderr: 'stderr'"),
					ContainSubstring("invalid character 'i' looking for beginning of value"))),
			)
		})
	})

	Context("when the external service adapter fails", func() {
		Context("when there is an operator error message and a user error message", func() {
			BeforeEach(func() {
				cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.ErrorExitCode), nil)
			})

			It("returns an UnknownFailureError", func() {
				commandError, ok := createBindingErr.(serviceadapter.UnknownFailureError)
				Expect(ok).To(BeTrue(), "error should be a Generic Error")
				Expect(commandError.Error()).To(Equal("stdout"))
			})

			It("logs the operator error message", func() {
				Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})
	})

	Context("when the external adapter fails with exit code 49", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.BindingAlreadyExistsErrorExitCode), nil)
		})

		It("returns the correct error", func() {
			Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.BindingAlreadyExistsError{}))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))
		})

		It("logs the operator error message", func() {
			Expect(logs).To(gbytes.Say("external service adapter exited with 49 at /thing: stdout: 'stdout', stderr: 'stderr'"))
		})
	})

	Context("when the external adapter fails with exit code 42", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.AppGuidNotProvidedErrorExitCode), nil)
		})

		It("returns the correct error", func() {
			Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.AppGuidNotProvidedError{}))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))
		})

		It("logs the operator error message", func() {
			Expect(logs).To(gbytes.Say("external service adapter exited with 42 at /thing: stdout: 'stdout', stderr: 'stderr'"))
		})
	})

	Context("when the external adapter fails with exit code 10", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
		})

		It("returns the correct error", func() {
			Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
			Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))
		})

		It("logs the operator error message", func() {
			Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'stdout', stderr: 'stderr'"))
		})
	})

	Context("when the external adapter fails to execute", func() {
		err := errors.New("oops")

		BeforeEach(func() {
			cmdRunner.RunReturns(nil, nil, nil, err)
		})

		It("returns an error", func() {
			Expect(createBindingErr).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
		})
	})

	When("UsingStdin is set to true", func() {
		BeforeEach(func() {
			a.UsingStdin = true

			cmdRunner.RunWithInputParamsReturns([]byte(`{
  				"credentials": {
  					"username": "user1",
  					"password": "reallysecret"
  				},
  				"syslog_drain_url": "some-url",
  				"route_service_url": "route-url"
  			}`), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
		})

		It("invokes external binding creator with serialised parameters in the stdin", func() {
			Expect(cmdRunner.RunCallCount()).To(Equal(0))
			Expect(cmdRunner.RunWithInputParamsCallCount()).To(Equal(1))
			actualInputParams, argsPassed := cmdRunner.RunWithInputParamsArgsForCall(0)
			Expect(argsPassed).To(ConsistOf(
				externalBinPath,
				"create-binding",
			))
			Expect(actualInputParams).To(Equal(sdk.InputParams{
				CreateBinding: sdk.CreateBindingJSONParams{
					BindingId:         bindingID,
					Manifest:          string(manifest),
					BoshVms:           toJson(deploymentTopology),
					RequestParameters: toJson(requestParams),
					Secrets:           toJson(secrets),
					DNSAddresses:      toJson(dnsAddresses),
				},
			}))
		})

		Context("when the external adapter returns an invalid binding", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("invalid json"), []byte("stderr"), intPtr(serviceadapter.SuccessExitCode), nil)
			})

			It("returns the service-specific binding output", func() {
				Expect(createBindingErr).To(
					MatchError(And(
						ContainSubstring("external service adapter returned invalid JSON"),
						ContainSubstring("stdout: 'invalid json', stderr: 'stderr'"),
						ContainSubstring("invalid character 'i' looking for beginning of value"))),
				)
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is an operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					commandError, ok := createBindingErr.(serviceadapter.UnknownFailureError)
					Expect(ok).To(BeTrue(), "error should be a Generic Error")
					Expect(commandError.Error()).To(Equal("stdout"))

					By("logging a message")
					Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'stdout', stderr: 'stderr'"))
				})
			})
		})

		Context("when the external adapter fails with exit code 49", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.BindingAlreadyExistsErrorExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.BindingAlreadyExistsError{}))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))

				By("logging a message")
				Expect(logs).To(gbytes.Say("external service adapter exited with 49 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})

		Context("when the external adapter fails with exit code 42", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.AppGuidNotProvidedErrorExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.AppGuidNotProvidedError{}))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))

				By("logging a message")
				Expect(logs).To(gbytes.Say("external service adapter exited with 42 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})

		Context("when the external adapter fails with exit code 10", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("stdout"), []byte("stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns the correct error", func() {
				Expect(createBindingErr).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stdout"))
				Expect(createBindingErr.Error()).NotTo(ContainSubstring("stderr"))

				By("logging a message")
				Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'stdout', stderr: 'stderr'"))
			})
		})

		Context("when the external adapter fails to execute", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns(nil, nil, nil, errors.New("oops"))
			})

			It("returns an error", func() {
				Expect(createBindingErr).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
			})
		})
	})
})
