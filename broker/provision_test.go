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
	"net/http"

	"github.com/pivotal-cf/on-demand-service-broker/cf"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Provisioning", func() {

	var (
		planID          string
		errandName      string
		errandInstance  string
		errandName2     string
		errandInstance2 string

		serviceSpec  brokerapi.ProvisionedServiceSpec
		provisionErr error

		organizationGUID       = "a-cf-org"
		spaceGUID              = "a-cf-space"
		instanceID             = "some-instance-id"
		jsonParams             []byte
		jsonContext            []byte
		arbParams              map[string]interface{}
		arbContext             map[string]interface{}
		requestMaintenanceInfo brokerapi.MaintenanceInfo

		asyncAllowed = true
		deployTaskID int

		provisionDetails brokerapi.ProvisionDetails
	)

	BeforeEach(func() {
		planID = existingPlanID
		asyncAllowed = true
		deployTaskID = 123
		requestMaintenanceInfo = brokerapi.MaintenanceInfo{}

		arbParams = map[string]interface{}{"foo": "bar"}
		arbContext = map[string]interface{}{"platform": "cloudfoundry", "space_guid": "final"}

		var err error
		jsonParams, err = json.Marshal(arbParams)
		Expect(err).NotTo(HaveOccurred())
		jsonContext, err = json.Marshal(arbContext)
		Expect(err).NotTo(HaveOccurred())
		boshClient.GetDeploymentReturns(nil, false, nil)
		provisionDetails = brokerapi.ProvisionDetails{
			PlanID:           planID,
			RawContext:       jsonContext,
			RawParameters:    jsonParams,
			OrganizationGUID: organizationGUID,
			SpaceGUID:        spaceGUID,
			ServiceID:        serviceOfferingID,
			MaintenanceInfo:  requestMaintenanceInfo,
		}
	})

	JustBeforeEach(func() {
		b = createDefaultBroker()
	})

	Context("when bosh deploys the release successfully", func() {
		var (
			newlyGeneratedManifest []byte
		)

		BeforeEach(func() {
			newlyGeneratedManifest = []byte("a newly generated manifest")
			fakeDeployer.CreateReturns(deployTaskID, newlyGeneratedManifest, nil)
		})

		It("returns expected operation data", func() {
			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)
			Expect(provisionErr).NotTo(HaveOccurred())

			By("reporting that the provisioning was asynchronous", func() {
				Expect(serviceSpec.IsAsync).To(BeTrue())
			})

			By("invoking the deployer", func() {
				Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
				actualDeploymentName, actualPlan, actualRequestParams, actualBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
				Expect(actualRequestParams).To(Equal(map[string]interface{}{
					"plan_id":           planID,
					"context":           arbContext,
					"parameters":        arbParams,
					"organization_guid": organizationGUID,
					"space_guid":        spaceGUID,
					"service_id":        serviceOfferingID,
					"maintenance_info":  map[string]interface{}{},
				}))
				Expect(actualPlan).To(Equal(planID))
				Expect(actualDeploymentName).To(Equal(deploymentName(instanceID)))
				Expect(actualBoshContextID).To(BeEmpty())
			})

			var operationData broker.OperationData
			Expect(json.Unmarshal([]byte(serviceSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData.BoshTaskID).To(Equal(deployTaskID))
			Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate))
			Expect(operationData.PlanID).To(BeEmpty())
			Expect(operationData.BoshContextID).To(BeEmpty())
		})

		Context("Handling dashboard url generation", func() {
			It("includes the dashboard url when implemented by the adapter", func() {
				serviceAdapter.GenerateDashboardUrlReturns("http://google.com", nil)

				serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

				Expect(serviceSpec.DashboardURL).To(Equal("http://google.com"))
				Expect(provisionErr).NotTo(HaveOccurred())

				By("invoking the adapter with the right arguments", func() {
					Expect(serviceAdapter.GenerateDashboardUrlCallCount()).To(Equal(1))
					instanceID, plan, boshManifest, _ := serviceAdapter.GenerateDashboardUrlArgsForCall(0)
					Expect(instanceID).To(Equal(instanceID))
					expectedProperties := sdk.Properties{"super": "no", "a_global_property": "global_value", "some_other_global_property": "other_global_value"}
					Expect(plan).To(Equal(sdk.Plan{
						Properties:     expectedProperties,
						InstanceGroups: existingPlan.InstanceGroups,
						Update:         existingPlan.Update,
					}))
					Expect(boshManifest).To(Equal(newlyGeneratedManifest))
				})
			})

			It("doesn't use dashboard url when adapter has not implemented the dashboard url", func() {
				serviceAdapter.GenerateDashboardUrlReturns("", serviceadapter.NewNotImplementedError("no dashboard!"))

				serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

				Expect(serviceSpec.DashboardURL).To(BeEmpty())
				Expect(provisionErr).NotTo(HaveOccurred())
			})

			It("includes a standard message when adapter returns a generic error", func() {
				serviceAdapter.GenerateDashboardUrlReturns("", errors.New("fooo"))

				serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

				Expect(logBuffer.String()).To(ContainSubstring("foo"))

				Expect(provisionErr).To(MatchError(
					SatisfyAll(
						ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
						MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
						ContainSubstring("service: a-cool-redis-service"),
						ContainSubstring("service-instance-guid: %s", instanceID),
						ContainSubstring("operation: create"),
						ContainSubstring("task-id: %d", deployTaskID),
					),
				))
			})

			It("includes the error when adapter errors with an AdapterCommandError", func() {
				serviceAdapter.GenerateDashboardUrlReturns("",
					serviceadapter.NewUnknownFailureError(
						"it failed, but all is not lost dear user",
					),
				)
				serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

				Expect(provisionErr).To(MatchError("it failed, but all is not lost dear user"))
			})
		})
	})

	Context("when the plan has post-deploy lifecycle errands", func() {
		BeforeEach(func() {
			planID = "post-deploy-errand-plan-id"
			errandName = "health-check"
			errandInstance = "post-deploy-instance-group-name/0"

			errandName2 = "health-check-2"
			errandInstance2 = "post-deploy-instance-group-name/0"

			postDeployErrandPlan := config.Plan{
				ID: planID,
				LifecycleErrands: &sdk.LifecycleErrands{
					PostDeploy: []sdk.Errand{{
						Name:      errandName,
						Instances: []string{errandInstance},
					}, {
						Name:      errandName2,
						Instances: []string{errandInstance2},
					}},
				},
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:               "post-deploy-instance-group-name",
						VMType:             "post-deploy-vm-type",
						PersistentDiskType: "post-deploy-disk-type",
						Instances:          101,
						Networks:           []string{"post-deploy-network"},
						AZs:                []string{"post-deploy-az"},
					},
				},
			}

			serviceCatalog.Plans = config.Plans{postDeployErrandPlan}
			provisionDetails.PlanID = planID
		})

		It("returns the correct operation data", func() {
			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).NotTo(HaveOccurred())

			var data broker.OperationData
			err := json.Unmarshal([]byte(serviceSpec.OperationData), &data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data.BoshContextID).NotTo(BeEmpty())
			Expect(data.Errands[0].Name).To(Equal(errandName))
			Expect(data.Errands[0].Instances).To(Equal([]string{errandInstance}))
			Expect(data.Errands[1].Name).To(Equal(errandName2))
			Expect(data.Errands[1].Instances).To(Equal([]string{errandInstance2}))

			By("calling the deployer with a bosh context id", func() {
				Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
				_, _, _, actualBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
				Expect(actualBoshContextID).NotTo(BeEmpty())
			})
		})

		It("calls the deployer with a different bosh context id when provision is called again", func() {

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)
			_, secondProvisionErr := b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(secondProvisionErr).NotTo(HaveOccurred())
			Expect(fakeDeployer.CreateCallCount()).To(Equal(2))
			_, _, _, firstBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
			Expect(firstBoshContextID).NotTo(BeNil())

			_, _, _, secondBoshContextID, _ := fakeDeployer.CreateArgsForCall(1)
			Expect(secondBoshContextID).NotTo(Equal(firstBoshContextID))
		})

	})

	Context("when the plan has a pre-delete lifecycle errand", func() {
		BeforeEach(func() {
			planID = "colocated-pre-delete-errand-plan-id"
			errandName = "cleanup-errand"
			errandInstance = "pre-delete-instance-group-name/0"

			preDeleteErrandPlan := config.Plan{
				ID: planID,
				LifecycleErrands: &sdk.LifecycleErrands{
					PreDelete: []sdk.Errand{{
						Name:      errandName,
						Instances: []string{errandInstance},
					}},
				},
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:               "pre-delete-instance-group-name",
						VMType:             "pre-delete-vm-type",
						PersistentDiskType: "pre-delete-disk-type",
						Instances:          101,
						Networks:           []string{"pre-delete-network"},
						AZs:                []string{"pre-delete-az"},
					},
				},
			}

			instanceID = "pre-delete-instance-group-name"

			serviceCatalog.Plans = config.Plans{preDeleteErrandPlan}
			provisionDetails.PlanID = planID
		})

		It("returns the correct operation data", func() {
			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).NotTo(HaveOccurred())
			var data broker.OperationData
			err := json.Unmarshal([]byte(serviceSpec.OperationData), &data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data.BoshContextID).NotTo(BeEmpty())
		})
	})

	Context("when invalid json params are provided by the broker api", func() {
		It("wraps the returns a raw params invalid error", func() {
			jsonParams = []byte("not valid json")
			provisionDetails.RawParameters = jsonParams

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(Equal(brokerapi.ErrRawParamsInvalid))
		})
	})

	Context("when no arbitrary params are passed by user", func() {
		It("no arbitrary params are passed to the adapter", func() {
			jsonParams = []byte{}
			provisionDetails.RawParameters = jsonParams

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).NotTo(HaveOccurred())
			_, _, actualRequestParams, _, _ := fakeDeployer.CreateArgsForCall(0)
			Expect(actualRequestParams["parameters"]).To(HaveLen(0))
			Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
		})
	})

	Context("when a deployment has a generic error", func() {
		It("errors with a standard message", func() {
			fakeDeployer.CreateReturns(0, []byte{}, fmt.Errorf("fooo"))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(logBuffer.String()).To(ContainSubstring("error: fooo"))
			Expect(provisionErr).To(MatchError(
				SatisfyAll(
					ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
					MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
					ContainSubstring("service: a-cool-redis-service"),
					ContainSubstring("service-instance-guid: %s", instanceID),
					ContainSubstring("operation: create"),
					Not(ContainSubstring("task-id: %d", deployTaskID)),
				),
			))
		})
	})

	Context("when getting the manifest has a bosh request error", func() {
		It("logs the error and returns try again error", func() {
			boshClient.GetDeploymentReturns([]byte{}, false, boshdirector.NewRequestError(
				fmt.Errorf("network timeout"),
			))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(logBuffer.String()).To(ContainSubstring("error: could not get manifest: network timeout"))
			Expect(provisionErr).To(MatchError(ContainSubstring("Currently unable to create service instance, please try again later")))
		})

	})

	Context("when a deploy has a bosh request error", func() {
		It("logs the error and returns try again error", func() {
			fakeDeployer.CreateReturns(0, []byte{}, boshdirector.NewRequestError(
				fmt.Errorf("error deploying instance: network timeout"),
			))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(logBuffer.String()).To(ContainSubstring("error: error deploying instance: network timeout"))
			Expect(provisionErr).To(MatchError(ContainSubstring("Currently unable to create service instance, please try again later")))
		})
	})

	Context("when a deployment has a user displayable error", func() {
		It("logs the error and returns the error", func() {
			fakeDeployer.CreateReturns(0, []byte{}, broker.NewDisplayableError(fmt.Errorf("user message"), fmt.Errorf("operator message")))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(logBuffer.String()).To(ContainSubstring("error: operator message"))
			Expect(provisionErr).To(MatchError(ContainSubstring("user message")))
		})
	})

	Context("when the deploy returns an adapter error with a user message", func() {
		It("returns the user error", func() {
			var err = serviceadapter.NewUnknownFailureError("it failed, but all is not lost dear user")
			fakeDeployer.CreateReturns(0, nil, err)

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(Equal(err))
		})
	})

	Context("when the deploy returns an adapter error with no message", func() {
		It("returns a generic error", func() {
			var err = serviceadapter.NewUnknownFailureError("")
			fakeDeployer.CreateReturns(0, nil, err)

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(MatchError(ContainSubstring(
				"There was a problem completing your request. Please contact your operations team providing the following information:",
			)))
		})
	})

	Context("when a provision of an already provisioned instance is triggered", func() {
		It("returns an error", func() {
			boshClient.GetDeploymentReturns([]byte(`manifest: true`), true, nil)

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(Equal(brokerapi.ErrInstanceAlreadyExists))
		})
	})

	Context("when the async allowed flag is false", func() {
		It("returns a  error", func() {
			asyncAllowed = false

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(Equal(brokerapi.ErrAsyncRequired))
		})
	})

	Context("when count instances of plan fails", func() {
		It("returns error that includes a standard message, the broker request ID, service name, service instance GUID and operation type", func() {
			cfClient.CountInstancesOfServiceOfferingReturns(nil, fmt.Errorf("count fail"))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(fakeDeployer.CreateCallCount()).To(BeZero())

			Expect(logBuffer.String()).To(ContainSubstring("count fail"))
			Expect(provisionErr).To(MatchError(
				SatisfyAll(
					ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:"),
					MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
					ContainSubstring("service: a-cool-redis-service"),
					ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
					ContainSubstring("operation: create"),
					Not(ContainSubstring("task-id:")),
				),
			))
		})
	})

	Context("when CF Integration is disabled", func() {
		It("succeeds", func() {
			noopCFClient := noopservicescontroller.New()
			broker, err := createBroker([]broker.StartupChecker{}, noopCFClient)
			Expect(err).NotTo(HaveOccurred())
			serviceSpec, provisionErr = broker.Provision(
				context.Background(),
				instanceID,
				brokerapi.ProvisionDetails{
					PlanID:           planID,
					RawParameters:    jsonParams,
					OrganizationGUID: organizationGUID,
					SpaceGUID:        spaceGUID,
					ServiceID:        serviceOfferingID,
				},
				asyncAllowed,
			)
			Expect(provisionErr).NotTo(HaveOccurred())
		})

	})

	Context("when plan id given is not configured", func() {
		It("return an error", func() {
			provisionDetails.PlanID = "wrong-plan"
			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)
			Expect(provisionErr).To(MatchError("plan wrong-plan not found"))
		})
	})

	Context("when bosh can't be reached", func() {
		It("returns an error", func() {
			boshClient.GetInfoReturns(boshdirector.Info{}, errors.New("foo"))

			serviceSpec, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).To(MatchError("Currently unable to create service instance, please try again later"))
		})
	})

	Context("when plan schemas are not enabled", func() {
		It("the json schemas are not requested", func() {
			fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
			brokerConfig.EnablePlanSchemas = false
			broker := createBrokerWithAdapter(fakeAdapter)
			serviceSpec, provisionErr = broker.Provision(
				context.Background(),
				instanceID,
				brokerapi.ProvisionDetails{
					PlanID:           planID,
					RawContext:       jsonContext,
					RawParameters:    jsonParams,
					OrganizationGUID: organizationGUID,
					SpaceGUID:        spaceGUID,
					ServiceID:        serviceOfferingID,
				},
				asyncAllowed,
			)

			Expect(provisionErr).NotTo(HaveOccurred())
			Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(0))
		})
	})

	Context("when plan schemas are enabled", func() {
		var err error
		var broker *broker.Broker
		var fakeAdapter *brokerfakes.FakeServiceAdapterClient

		BeforeEach(func() {
			fakeAdapter = new(brokerfakes.FakeServiceAdapterClient)
			fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
			brokerConfig.EnablePlanSchemas = true
			broker = createBrokerWithAdapter(fakeAdapter)
		})

		JustBeforeEach(func() {
			serviceSpec, provisionErr = broker.Provision(
				context.Background(),
				instanceID,
				brokerapi.ProvisionDetails{
					PlanID:           planID,
					RawContext:       jsonContext,
					RawParameters:    jsonParams,
					OrganizationGUID: organizationGUID,
					SpaceGUID:        spaceGUID,
					ServiceID:        serviceOfferingID,
				},
				asyncAllowed,
			)
		})

		Context("if the service adapter fails", func() {
			BeforeEach(func() {
				fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, errors.New("oops"))
			})

			It("returns an error", func() {
				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("oops"))
			})

		})

		Context("if the service adapter does not implement plan schemas", func() {
			BeforeEach(func() {
				serviceAdapterError := serviceadapter.NewNotImplementedError("no.")
				fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, serviceAdapterError)
			})

			It("returns an error", func() {
				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
				Expect(logBuffer.String()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
			})
		})

		Context("when the provision request params are not valid", func() {
			BeforeEach(func() {
				arbParams = map[string]interface{}{
					"this-is": "clearly-wrong",
				}
				jsonParams, err = json.Marshal(arbParams)
				Expect(err).NotTo(HaveOccurred())
			})

			It("requests the json schemas from the service adapter", func() {
				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("Additional property this-is is not allowed"))
				Expect(provisionErr).To(BeAssignableToTypeOf(&brokerapi.FailureResponse{}))

				actualErr := provisionErr.(*brokerapi.FailureResponse)
				Expect(actualErr.ValidatedStatusCode(nil)).To(Equal(http.StatusBadRequest))
			})

			It("fails", func() {
				Expect(provisionErr).To(HaveOccurred())
			})
		})

		Context("when the provision request params are empty", func() {
			var err error

			BeforeEach(func() {
				jsonParams = []byte{}
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
				Expect(serviceSpec.OperationData).To(ContainSubstring("\"OperationType\":\"create\""))
			})
		})

		Context("when the provision request params are valid", func() {
			var err error

			BeforeEach(func() {
				arbParams = map[string]interface{}{
					"auto_create_topics":         true,
					"default_replication_factor": 55,
				}
				jsonParams, err = json.Marshal(arbParams)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
				Expect(serviceSpec.OperationData).To(ContainSubstring("\"OperationType\":\"create\""))
			})
		})

		Context("when the schema allows additional properties", func() {
			var err error

			BeforeEach(func() {
				arbParams = map[string]interface{}{
					"foo": true,
					"bar": 55,
				}
				jsonParams, err = json.Marshal(arbParams)
				Expect(err).NotTo(HaveOccurred())
				fakeAdapter.GeneratePlanSchemaReturns(schemaWithAdditionalPropertiesAllowedFixture, nil)
			})

			It("succeeds", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
				Expect(serviceSpec.OperationData).To(ContainSubstring("\"OperationType\":\"create\""))
			})
		})

		Context("when the schema has required properties", func() {
			var err error

			BeforeEach(func() {
				arbParams = map[string]interface{}{
					"foo": true,
					"bar": 55,
				}
				jsonParams, err = json.Marshal(arbParams)
				Expect(err).NotTo(HaveOccurred())
				fakeAdapter.GeneratePlanSchemaReturns(schemaWithRequiredPropertiesFixture, nil)
			})

			It("reports the required error", func() {
				Expect(provisionErr).To(MatchError(ContainSubstring("auto_create_topics is required")))
			})
		})
	})

	Describe("plan quotas", func() {
		var provisionErr error

		BeforeEach(func() {
			planID = existingPlanID
			asyncAllowed = true

			arbParams = map[string]interface{}{"foo": "bar"}
			arbContext = map[string]interface{}{"platform": "cloudfoundry", "space_guid": "final"}

			var err error
			jsonParams, err = json.Marshal(arbParams)
			Expect(err).NotTo(HaveOccurred())
			jsonContext, err = json.Marshal(arbContext)
			Expect(err).NotTo(HaveOccurred())
			boshClient.GetDeploymentReturns(nil, false, nil)

		})

		deployWithQuotas := func(q quotaCase, planToDeploy string, existingInstanceCount int) error {
			planCounts := map[cf.ServicePlan]int{
				cfServicePlan("1234", existingPlanID, "url", "name"): existingInstanceCount,
			}
			cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)

			plan := existingPlan
			plan.Quotas = config.Quotas{}
			plan.ResourceCosts = map[string]int{"ips": 1, "memory": 1}
			catalogWithResourceQuotas := serviceCatalog

			// set up quotas
			if q.PlanInstanceLimit != nil {
				plan.Quotas.ServiceInstanceLimit = q.PlanInstanceLimit
			}
			if len(q.PlanResourceLimits) > 0 {
				plan.Quotas.ResourceLimits = q.PlanResourceLimits
			}
			if len(q.GlobalResourceLimits) > 0 {
				catalogWithResourceQuotas.GlobalQuotas.ResourceLimits = q.GlobalResourceLimits
			}
			if q.GlobalInstanceLimit != nil {
				catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = q.GlobalInstanceLimit
			} else {
				limit := 50
				catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = &limit
			}

			catalogWithResourceQuotas.Plans = config.Plans{plan, secondPlan}
			fakeDeployer = new(brokerfakes.FakeDeployer)
			b = createBrokerWithServiceCatalog(catalogWithResourceQuotas)

			_, provisionErr = b.Provision(
				context.Background(),
				instanceID,
				brokerapi.ProvisionDetails{
					PlanID:           planToDeploy,
					RawContext:       jsonContext,
					RawParameters:    jsonParams,
					OrganizationGUID: organizationGUID,
					SpaceGUID:        spaceGUID,
					ServiceID:        serviceOfferingID,
				},
				asyncAllowed,
			)

			return provisionErr
		}

		Context("when quotas are not enabled", func() {
			var deployErr error
			BeforeEach(func() {
				deployErr = deployWithQuotas(
					quotaCase{},
					existingPlanID, 0)
			})

			It("deploy succeeds", func() {
				Expect(deployErr).NotTo(HaveOccurred())
			})

			It("plan instance count is not checked", func() {
				Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(0))
			})
		})

		It("deploy succeeds when no quotas are reached", func() {
			aLot := 99
			deployErr := deployWithQuotas(
				quotaCase{nil, nil, &aLot, &aLot},
				existingPlanID, 10)
			Expect(deployErr).NotTo(HaveOccurred())
		})

		Context("instance limits", func() {
			It("deploy fails when plan instance limit is reached", func() {
				planInstanceLimit := 1
				provisionErr = deployWithQuotas(
					quotaCase{nil, nil, nil, &planInstanceLimit},
					existingPlanID,
					1)

				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("plan instance limit exceeded for service ID: service-id. Total instances: 1"))
			})

			It("deploy fails when global instance limit is reached", func() {
				globalInstanceLimit := 10
				provisionErr = deployWithQuotas(
					quotaCase{nil, nil, &globalInstanceLimit, nil},
					existingPlanID,
					10)

				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("global instance limit exceeded for service ID: service-id. Total instances: 10"))
			})
		})

		Context("resource limits", func() {
			It("deploy fails when plan resource limit is reached", func() {
				planResourceLimits := map[string]int{"ips": 1} // plan costs 1 IP per instance
				provisionErr = deployWithQuotas(
					quotaCase{nil, planResourceLimits, nil, nil},
					existingPlanID,
					1)

				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("plan quotas [ips: (limit 1, used 1, requires 1)] would be exceeded by this deployment"))
			})

			It("deploy fails when global resource limit is reached", func() {
				globalResourceLimits := map[string]int{"ips": 5} // plan costs 1 IP per instance
				provisionErr = deployWithQuotas(
					quotaCase{globalResourceLimits, nil, nil, nil},
					existingPlanID,
					5)

				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring("global quotas [ips: (limit 5, used 5, requires 1)] would be exceeded by this deployment"))
			})

			It("succeeds when plan resource quota is set and has been reached but there is no instance count limit", func() {
				planResourceLimits := map[string]int{"ips": 5} // plan costs 1 IP per instance
				provisionErr = deployWithQuotas(
					quotaCase{nil, planResourceLimits, nil, nil},
					secondPlanID,
					5)

				Expect(provisionErr).NotTo(HaveOccurred())
			})
		})

		Describe("when all quotas are reached simultaneously", func() {
			var deployErr error
			BeforeEach(func() {
				planResourceLimits := map[string]int{"ips": 1, "memory": 1} // plan uses 1 IP and 1 memory
				globalResourceLimits := map[string]int{"ips": 1}
				globalInstanceLimit := 1
				planInstanceLimit := 1

				deployErr = deployWithQuotas(
					quotaCase{globalResourceLimits, planResourceLimits, &globalInstanceLimit, &planInstanceLimit},
					existingPlanID,
					1)
			})

			It("deploy fails", func() {
				Expect(deployErr).To(HaveOccurred())
				Expect(deployErr.Error()).To(SatisfyAll(
					ContainSubstring("plan instance limit exceeded for service ID: service-id. Total instances: 1"),
					ContainSubstring("global instance limit exceeded for service ID: service-id. Total instances: 1"),
					ContainSubstring("global quotas [ips: (limit 1, used 1, requires 1)] would be exceeded by this deployment"),
					ContainSubstring("plan quotas ["),
					ContainSubstring("ips: (limit 1, used 1, requires 1)"),
					ContainSubstring("memory: (limit 1, used 1, requires 1)"),
					ContainSubstring("] would be exceeded by this deployment"),
				))
			})
		})

		Describe("when global resource quotas and plan resource quotas are set, and both have been reached", func() {
			It("provisions successfully when the plan doesn't count against the global quota", func() {
				planResourceLimits := map[string]int{"ips": 1, "memory": 1} // plan uses 1 IP and 1 memory
				globalResourceLimits := map[string]int{"ips": 1}
				provisionErr = deployWithQuotas(
					quotaCase{globalResourceLimits, planResourceLimits, nil, nil},
					secondPlanID,
					1)

				Expect(provisionErr).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Maintenance info", func() {
		BeforeEach(func() {
			newlyGeneratedManifest := []byte("a newly generated manifest")

			serviceCatalog.MaintenanceInfo = &config.MaintenanceInfo{
				Public: map[string]string{
					"edition": "gold millennium",
				},
				Private: map[string]string{},
			}
			fakeDeployer.CreateReturns(deployTaskID, newlyGeneratedManifest, nil)
		})

		It("succeeds when maintenanceInfo.Checker succeeds", func() {

			provisionDetails.MaintenanceInfo = brokerapi.MaintenanceInfo{
				Version: "1.2.3",
				Public: map[string]string{
					"edition": "gold millennium",
				},
				Private: "secret:password;",
			}

			_, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(provisionErr).ToNot(HaveOccurred())
			Expect(fakeMaintenanceInfoChecker.CheckCallCount()).To(Equal(1), "Check was not called")
			actualPlanID, actualMaintenanceInfo, actualServiceCatalog, _ := fakeMaintenanceInfoChecker.CheckArgsForCall(0)
			Expect(actualMaintenanceInfo).To(Equal(provisionDetails.MaintenanceInfo))
			Expect(actualPlanID).To(Equal(provisionDetails.PlanID))

			serviceCatalog, _ := b.Services(nil)
			Expect(actualServiceCatalog).To(Equal(serviceCatalog))
		})

		It("fails when the maintenanceInfo.Checker fails", func() {
			fakeMaintenanceInfoChecker.CheckReturns(fmt.Errorf("nope"))

			_, provisionErr = b.Provision(context.Background(), instanceID, provisionDetails, asyncAllowed)

			Expect(fakeMaintenanceInfoChecker.CheckCallCount()).To(Equal(1), "Check was not called")
			Expect(provisionErr).To(MatchError(ContainSubstring("nope")))
		})
	})
})
