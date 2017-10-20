// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("Bind", func() {
	var (
		instanceID             = "a-very-impressive-instance"
		bindingID              = "binding-id"
		serviceDeploymentName  = deploymentName(instanceID)
		adapterBindingResponse = sdk.Binding{
			Credentials:     map[string]interface{}{"foo": "bar"},
			RouteServiceURL: "route",
			SyslogDrainURL:  "syslog",
		}
		boshVms             = bosh.BoshVMs{"redis-server": []string{"an.ip"}}
		actualManifest      = []byte("valid manifest")
		arbitraryParameters = map[string]interface{}{"arb": "param"}

		bindResult brokerapi.Binding
		bindErr    error

		bindRequest brokerapi.BindDetails
	)

	BeforeEach(func() {
		boshClient.VMsReturns(boshVms, nil)
		boshClient.GetDeploymentReturns(actualManifest, true, nil)
		serviceAdapter.CreateBindingReturns(adapterBindingResponse, nil)

		serialisedArbitraryParameters, err := json.Marshal(arbitraryParameters)
		Expect(err).NotTo(HaveOccurred())
		bindRequest = brokerapi.BindDetails{
			AppGUID:   "app_guid",
			PlanID:    "plan_id",
			ServiceID: "service_id",
			BindResource: &brokerapi.BindResource{
				AppGuid: "app_guid",
			},
			RawParameters: serialisedArbitraryParameters,
		}
	})

	Context("when CF integration is disabled", func() {

		It("returns that is deprovisioning asynchronously", func() {

			boshInfo = createBOSHInfoWithMajorVersion(
				broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
				boshdirector.VersionType("semver"),
			)
			b, brokerCreationErr = createBroker(boshInfo, noopservicescontroller.New())
			Expect(brokerCreationErr).NotTo(HaveOccurred())

			bindResult, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest)
			Expect(serviceAdapter.CreateBindingCallCount()).To(Equal(1))
			passedBindingID, passedVms, passedManifest, passedRequestParameters, _ := serviceAdapter.CreateBindingArgsForCall(0)
			Expect(passedBindingID).To(Equal(bindingID))
			Expect(passedVms).To(Equal(boshVms))
			Expect(passedManifest).To(Equal(actualManifest))
			Expect(passedRequestParameters).To(Equal(map[string]interface{}{
				"app_guid":   "app_guid",
				"plan_id":    "plan_id",
				"service_id": "service_id",
				"bind_resource": map[string]interface{}{
					"app_guid": "app_guid",
				},
				"parameters": arbitraryParameters,
			}))
		})
	})

	Context("with default CF client", func() {

		JustBeforeEach(func() {
			b = createDefaultBroker()
			bindResult, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest)
		})

		It("asks bosh for VMs from a deployment named by the manifest generator", func() {
			Expect(boshClient.VMsCallCount()).To(Equal(1))
			actualServiceDeploymentName, _ := boshClient.VMsArgsForCall(0)
			Expect(actualServiceDeploymentName).To(Equal(serviceDeploymentName))
		})

		It("creates the binding using the bosh topology and admin credentials", func() {
			Expect(serviceAdapter.CreateBindingCallCount()).To(Equal(1))
			passedBindingID, passedVms, passedManifest, passedRequestParameters, _ := serviceAdapter.CreateBindingArgsForCall(0)
			Expect(passedBindingID).To(Equal(bindingID))
			Expect(passedVms).To(Equal(boshVms))
			Expect(passedManifest).To(Equal(actualManifest))
			Expect(passedRequestParameters).To(Equal(map[string]interface{}{
				"app_guid":   "app_guid",
				"plan_id":    "plan_id",
				"service_id": "service_id",
				"bind_resource": map[string]interface{}{
					"app_guid": "app_guid",
				},
				"parameters": arbitraryParameters,
			}))
		})

		It("returns the credentials, syslog drain url, and route service url produced by the service adapter", func() {
			Expect(bindResult).To(Equal(brokerapi.Binding{
				Credentials:     adapterBindingResponse.Credentials,
				SyslogDrainURL:  adapterBindingResponse.SyslogDrainURL,
				RouteServiceURL: adapterBindingResponse.RouteServiceURL,
			}))
		})

		It("does not error", func() {
			Expect(bindErr).NotTo(HaveOccurred())
		})

		It("logs using a request ID", func() {
			Expect(logBuffer.String()).To(MatchRegexp(fmt.Sprintf(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} service adapter will create binding with ID %s for instance %s`, bindingID, instanceID)))
		})

		Context("when the request cannot be converted to json", func() {
			BeforeEach(func() {
				bindRequest = brokerapi.BindDetails{
					AppGUID:   "app_guid",
					PlanID:    "plan_id",
					ServiceID: "service_id",
					BindResource: &brokerapi.BindResource{
						AppGuid: "app_guid",
					},
					RawParameters: []byte(`not valid json`),
				}
			})

			It("should return an error", func() {
				Expect(bindErr).To(HaveOccurred())
			})
		})

		Context("when binding to a non existent instance", func() {
			BeforeEach(func() {
				boshClient.VMsReturns(nil, boshdirector.DeploymentNotFoundError{})
			})

			It("returns a standard error message", func() {
				Expect(bindErr).To(Equal(brokerapi.ErrInstanceDoesNotExist))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("error binding: instance " + instanceID + ", not found"))
			})
		})

		Context("when bosh fails to get VMs", func() {
			BeforeEach(func() {
				boshClient.VMsReturns(nil, errors.New("oops"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(bindErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(bindErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(bindErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes a service instance guid", func() {
					Expect(bindErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(bindErr).To(MatchError(ContainSubstring(
						"operation: bind",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(bindErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("oops"))
			})
		})

		Context("when manifest is not found", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, nil)
			})

			It("returns an error", func() {
				Expect(bindErr).To(HaveOccurred())
			})
		})

		Context("when broker fails to fetch manifest", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, fmt.Errorf("problem fetching manifest"))
			})

			It("returns an error", func() {
				Expect(bindErr).To(HaveOccurred())
			})
		})

		Context("when bind has a bosh request error", func() {
			BeforeEach(func() {
				boshClient.VMsReturns(nil, boshdirector.NewRequestError(errors.New("bosh down.")))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("error: could not get deployment info: bosh down."))
			})

			It("returns the try again later error for the user", func() {
				Expect(bindErr).To(MatchError(ContainSubstring("Currently unable to bind service instance, please try again later")))
			})
		})

		Context("when the service adapter fails to create the binding", func() {
			Context("with a generic error", func() {
				Context("with no message for the user", func() {
					BeforeEach(func() {
						serviceAdapter.CreateBindingReturns(sdk.Binding{}, errors.New("binding fail"))
					})

					Describe("returned error", func() {
						It("includes a standard message", func() {
							Expect(bindErr).To(MatchError(ContainSubstring(
								"There was a problem completing your request. Please contact your operations team providing the following information:",
							)))
						})

						It("includes the broker request id", func() {
							Expect(bindErr).To(MatchError(MatchRegexp(
								`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
							)))
						})

						It("includes the service name", func() {
							Expect(bindErr).To(MatchError(ContainSubstring(
								"service: a-cool-redis-service",
							)))
						})

						It("includes a service instance guid", func() {
							Expect(bindErr).To(MatchError(ContainSubstring(
								fmt.Sprintf("service-instance-guid: %s", instanceID),
							)))
						})

						It("includes the operation type", func() {
							Expect(bindErr).To(MatchError(ContainSubstring(
								"operation: bind",
							)))
						})

						It("does NOT include the bosh task id", func() {
							Expect(bindErr).NotTo(MatchError(ContainSubstring(
								"task-id:",
							)))
						})
					})

					It("logs the error", func() {
						Expect(logBuffer.String()).To(ContainSubstring("binding fail"))
					})
				})

				Context("with an error message for the user", func() {
					var err = serviceadapter.NewUnknownFailureError("it failed, but all is not lost dear user")

					BeforeEach(func() {
						serviceAdapter.CreateBindingReturns(sdk.Binding{}, err)
					})

					It("returns the user error", func() {
						Expect(bindErr).To(Equal(err))
					})
				})
			})

			Context("with a binding already exists error", func() {
				BeforeEach(func() {
					serviceAdapter.CreateBindingReturns(sdk.Binding{}, serviceadapter.BindingAlreadyExistsError{})
				})

				It("returns a binding already exists error", func() {
					Expect(bindErr).To(Equal(brokerapi.ErrBindingAlreadyExists))
				})
			})

			Context("with the app_guid not provided", func() {
				BeforeEach(func() {
					serviceAdapter.CreateBindingReturns(sdk.Binding{}, serviceadapter.AppGuidNotProvidedError{})
				})

				It("returns an 'app guid not provided' error", func() {
					Expect(bindErr).To(Equal(brokerapi.ErrAppGuidNotProvided))
				})
			})
		})
	})
})
