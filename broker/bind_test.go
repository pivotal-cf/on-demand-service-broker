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
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
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
		arbitraryContext    = map[string]interface{}{"platform": "cloudfoundry"}
		actualDNSAddresses  map[string]string

		bindResult brokerapi.Binding
		bindErr    error

		bindRequest   brokerapi.BindDetails
		boshVariables []boshdirector.Variable
	)

	BeforeEach(func() {
		boshClient.VMsReturns(boshVms, nil)
		boshClient.GetDeploymentReturns(actualManifest, true, nil)
		serviceAdapter.CreateBindingReturns(adapterBindingResponse, nil)
		fakeSecretManager.ResolveManifestSecretsReturns(map[string]string{}, nil)

		serialisedArbitraryParameters, err := json.Marshal(arbitraryParameters)
		Expect(err).NotTo(HaveOccurred())
		serialisedContext, err := json.Marshal(arbitraryContext)
		Expect(err).NotTo(HaveOccurred())
		bindRequest = brokerapi.BindDetails{
			AppGUID:   "app_guid",
			PlanID:    "plan_id",
			ServiceID: "service_id",
			BindResource: &brokerapi.BindResource{
				AppGuid: "app_guid",
			},
			RawParameters: serialisedArbitraryParameters,
			RawContext:    serialisedContext,
		}

		boshVariables = []boshdirector.Variable{
			{Path: "/foo/bar", ID: "123asd"},
			{Path: "/some/path", ID: "456zxc"},
		}

		actualDNSAddresses = map[string]string{
			"config1": "some.dns.bosh",
			"config2": "some-other.dns.bosh",
		}
		boshClient.GetDNSAddressesReturns(actualDNSAddresses, nil)
	})

	Context("request ID", func() {
		It("generates a new request ID when no request ID is present in the ctx", func() {
			b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
			Expect(brokerCreationErr).NotTo(HaveOccurred())

			contextWithoutRequestID := context.Background()
			bindResult, bindErr = b.Bind(contextWithoutRequestID, instanceID, bindingID, bindRequest, false)

			Expect(logBuffer.String()).To(MatchRegexp(
				`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\]`))
		})

		It("does not generate a new uuid when one is provided through the ctx", func() {
			b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
			Expect(brokerCreationErr).NotTo(HaveOccurred())

			requestID := uuid.New()
			contextWithReqID := brokercontext.WithReqID(context.Background(), requestID)
			bindResult, bindErr = b.Bind(contextWithReqID, instanceID, bindingID, bindRequest, false)

			Expect(logBuffer.String()).To(ContainSubstring(requestID))
		})
	})

	Context("when CF integration is disabled", func() {
		It("returns that is provisioning asynchronously", func() {
			b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
			Expect(brokerCreationErr).NotTo(HaveOccurred())

			bindResult, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(serviceAdapter.CreateBindingCallCount()).To(Equal(1))
			passedBindingID, passedVms, passedManifest, passedRequestParameters, _, passedDNSAddresses, _ := serviceAdapter.CreateBindingArgsForCall(0)
			Expect(passedBindingID).To(Equal(bindingID))
			Expect(passedVms).To(Equal(boshVms))
			Expect(passedManifest).To(Equal(actualManifest))
			Expect(passedDNSAddresses).To(Equal(actualDNSAddresses))
			Expect(passedRequestParameters).To(Equal(map[string]interface{}{
				"app_guid":   "app_guid",
				"plan_id":    "plan_id",
				"service_id": "service_id",
				"bind_resource": map[string]interface{}{
					"app_guid": "app_guid",
				},
				"parameters": arbitraryParameters,
				"context":    arbitraryContext,
			}))
		})
	})

	Context("with default CF client", func() {
		var asyncAllowed bool

		BeforeEach(func() {
			asyncAllowed = false
		})

		JustBeforeEach(func() {
			b = createDefaultBroker()
			bindResult, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, asyncAllowed)
		})

		It("asks bosh for VMs from a deployment named by the manifest generator", func() {
			Expect(boshClient.VMsCallCount()).To(Equal(1))
			actualServiceDeploymentName, _ := boshClient.VMsArgsForCall(0)
			Expect(actualServiceDeploymentName).To(Equal(serviceDeploymentName))
		})

		It("creates the binding using the bosh topology, admin credentials and bosh dns addresses", func() {
			Expect(serviceAdapter.CreateBindingCallCount()).To(Equal(1))
			passedBindingID, passedVms, passedManifest, passedRequestParameters, _, passedDNSAddresses, _ := serviceAdapter.CreateBindingArgsForCall(0)
			Expect(passedBindingID).To(Equal(bindingID))
			Expect(passedVms).To(Equal(boshVms))
			Expect(passedManifest).To(Equal(actualManifest))
			Expect(passedDNSAddresses).To(Equal(actualDNSAddresses))
			Expect(passedRequestParameters).To(Equal(map[string]interface{}{
				"app_guid":   "app_guid",
				"plan_id":    "plan_id",
				"service_id": "service_id",
				"bind_resource": map[string]interface{}{
					"app_guid": "app_guid",
				},
				"parameters": arbitraryParameters,
				"context":    arbitraryContext,
			}))
		})

		It("returns the credentials, syslog drain url, and route service url produced by the service adapter", func() {
			Expect(bindResult).To(Equal(brokerapi.Binding{
				Credentials:     adapterBindingResponse.Credentials,
				SyslogDrainURL:  adapterBindingResponse.SyslogDrainURL,
				RouteServiceURL: adapterBindingResponse.RouteServiceURL,
			}))
		})

		It("returns synchronous response when asyncAllowed is false", func() {
			Expect(bindResult.IsAsync).To(BeFalse(), "returned unexpected async bind response")
		})

		Context("asyncAllowed is passed as true", func() {
			BeforeEach(func() {
				asyncAllowed = true
			})

			It("bind ignores asyncAllowed and returns synchronous response", func() {
				Expect(bindResult.IsAsync).To(BeFalse(), "returned unexpected async bind response")
			})
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
				boshClient.GetDeploymentReturns(nil, false, nil)
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

		Context("when bind has a bosh DNS request error", func() {
			BeforeEach(func() {
				boshClient.GetDNSAddressesReturns(nil, boshdirector.NewRequestError(errors.New("could not find link provider")))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("error: failed to get required DNS info: could not find link provider"))
			})

			It("returns generic error message to user", func() {
				Expect(bindErr).To(MatchError(ContainSubstring("There was a problem completing your request. Please")))
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

	It("does not validates the parameters passed to the bind request if plan schemas are not enabled", func() {
		fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
		fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
		brokerConfig.EnablePlanSchemas = false
		b = createBrokerWithAdapter(fakeAdapter)

		bindRequest := generateBindRequestWithParams(map[string]interface{}{
			"bind_auto_create_topics":         true,
			"bind_default_replication_factor": 5,
		})

		_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
		Expect(bindErr).NotTo(HaveOccurred())
		Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(0))
	})

	Context("when plan schemas are enabled", func() {
		BeforeEach(func() {
			brokerConfig.EnablePlanSchemas = true
		})

		It("validates the parameters passed to the bind request", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{
				"bind_auto_create_topics":         true,
				"bind_default_replication_factor": 5,
			})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).NotTo(HaveOccurred())
			Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
		})

		It("returns an error if the parameters are invalid", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{
				"bind_auto_create_topics": 1,
			})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(bindErr.Error()).To(ContainSubstring(
				"bind_auto_create_topics: Invalid type. Expected: boolean, given: integer",
			))
		})

		It("returns an error if the generated schema is not valid", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			badSchemaFixture := brokerapi.ServiceSchemas{
				Binding: brokerapi.ServiceBindingSchema{
					Create: brokerapi.Schema{Parameters: invalidSchema},
				},
			}
			fakeAdapter.GeneratePlanSchemaReturns(badSchemaFixture, nil)
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{
				"bind_auto_create_topics": true,
			})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(bindErr.Error()).To(ContainSubstring("failed validating schema - schema does not conform to JSON Schema spec"))
		})

		It("does not fail if no parameters are provided", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).NotTo(HaveOccurred())
			Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
		})

		It("returns an error if the service adapter fails", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, errors.New("oops"))
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(bindErr.Error()).To(ContainSubstring("oops"))
		})

		It("returns an error if the service adapter does not implement generate_plan_schemas", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			serviceAdapterError := serviceadapter.NewNotImplementedError("no.")
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, serviceAdapterError)
			b = createBrokerWithAdapter(fakeAdapter)

			bindRequest := generateBindRequestWithParams(map[string]interface{}{})

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(bindErr.Error()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
			Expect(logBuffer.String()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
		})
	})

	Describe("secret resolver", func() {
		var broker *broker.Broker

		BeforeEach(func() {
			broker = createDefaultBroker()
			boshClient.VariablesReturns(boshVariables, nil)
		})

		It("is called with manifest as param", func() {
			bindResult, bindErr = broker.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(serviceAdapter.CreateBindingCallCount()).To(Equal(1))
			Expect(fakeSecretManager.ResolveManifestSecretsCallCount()).To(Equal(1))
			manifest, deploymentVariables, _ := fakeSecretManager.ResolveManifestSecretsArgsForCall(0)
			Expect(manifest).To(Equal(actualManifest))
			Expect(deploymentVariables).To(Equal([]boshdirector.Variable{
				{Path: "/foo/bar", ID: "123asd"},
				{Path: "/some/path", ID: "456zxc"},
			}))
		})

		It("logs errors when cannot resolve secrets", func() {
			resolveError := errors.New("resolve error")
			fakeSecretManager.ResolveManifestSecretsReturns(nil, resolveError)
			_, bindErr = broker.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).NotTo(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("failed to resolve manifest secrets: %s", resolveError.Error())))
		})

		It("logs errors and the status of the feature flag if the adapter is called but it returns an error", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.CreateBindingReturns(sdk.Binding{}, errors.New("secrets needed"))
			brokerConfig.EnableSecureManifests = false

			b = createBrokerWithAdapter(fakeAdapter)

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring("resolve_secrets_at_bind was: false"))
			Expect(logBuffer.String()).To(ContainSubstring("secrets needed"))
		})

		It("logs but not fail when cannot retrieve the deployment variables ", func() {
			boshClient.VariablesReturns(nil, errors.New("oh noes"))
			bindResult, bindErr = broker.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(logBuffer.String()).To(ContainSubstring("failed to retrieve deployment variables"))
		})

		It("logs errors if the adapter is called but it returns an error", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.CreateBindingReturns(sdk.Binding{}, errors.New("secrets needed"))
			brokerConfig.EnableSecureManifests = true

			b = createBrokerWithAdapter(fakeAdapter)

			_, bindErr = b.Bind(context.Background(), instanceID, bindingID, bindRequest, false)
			Expect(bindErr).To(HaveOccurred())
			Expect(logBuffer.String()).ToNot(ContainSubstring("resolve_secrets_at_bind was:"))
			Expect(logBuffer.String()).To(ContainSubstring("secrets needed"))
		})
	})
})

func generateBindRequestWithParams(params map[string]interface{}) brokerapi.BindDetails {
	serialisedArbitraryParameters, err := json.Marshal(params)
	Expect(err).NotTo(HaveOccurred())
	return brokerapi.BindDetails{
		AppGUID:   "app_guid",
		PlanID:    existingPlanID,
		ServiceID: "service_id",
		BindResource: &brokerapi.BindResource{
			AppGuid: "app_guid",
		},
		RawParameters: serialisedArbitraryParameters,
	}
}
