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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("provisioning", func() {
	var (
		planID          string
		errandName      string
		errandInstance  string
		errandName2     string
		errandInstance2 string

		serviceSpec  brokerapi.ProvisionedServiceSpec
		provisionErr error

		organizationGUID = "a-cf-org"
		spaceGUID        = "a-cf-space"
		instanceID       = "some-instance-id"
		jsonParams       []byte
		jsonContext      []byte
		arbParams        map[string]interface{}
		arbContext       map[string]interface{}

		asyncAllowed = true
	)

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

	JustBeforeEach(func() {
		b = createDefaultBroker()
		serviceSpec, provisionErr = b.Provision(
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

	Context("when bosh deploys the release successfully", func() {
		var (
			newlyGeneratedManifest []byte
			deployTaskID           = 123
		)

		BeforeEach(func() {
			newlyGeneratedManifest = []byte("a newly generated manifest")
			fakeDeployer.CreateReturns(deployTaskID, newlyGeneratedManifest, nil)
		})

		It("does not error", func() {
			Expect(provisionErr).NotTo(HaveOccurred())
		})

		It("reports that the provisioning was asynchronous", func() {
			Expect(serviceSpec.IsAsync).To(BeTrue())
		})

		It("invokes the deployer", func() {
			Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
			actualDeploymentName, actualPlan, actualRequestParams, actualBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
			Expect(actualRequestParams).To(Equal(map[string]interface{}{
				"plan_id":           planID,
				"context":           arbContext,
				"parameters":        arbParams,
				"organization_guid": organizationGUID,
				"space_guid":        spaceGUID,
				"service_id":        serviceOfferingID,
			}))
			Expect(actualPlan).To(Equal(planID))
			Expect(actualDeploymentName).To(Equal(deploymentName(instanceID)))
			Expect(actualBoshContextID).To(BeEmpty())
		})

		It("returns operation data with bosh task ID and operation type", func() {
			var operationData broker.OperationData
			Expect(json.Unmarshal([]byte(serviceSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData.BoshTaskID).To(Equal(deployTaskID))
			Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate))
		})

		It("return operation data without add bosh context ID or plan ID", func() {
			var operationData broker.OperationData
			Expect(json.Unmarshal([]byte(serviceSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData.PlanID).To(BeEmpty())
			Expect(operationData.BoshContextID).To(BeEmpty())
		})

		It("invokes the adapter for the dashboard url, merging global and plan properties", func() {
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

		Context("adapter returns a dashboard url", func() {
			BeforeEach(func() {
				serviceAdapter.GenerateDashboardUrlReturns("http://google.com", nil)
			})

			It("returns the dashboard url", func() {
				Expect(serviceSpec.DashboardURL).To(Equal("http://google.com"))
			})

			It("does not error", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
			})
		})

		Context("adapter has not implemented the dashboard url", func() {
			BeforeEach(func() {
				serviceAdapter.GenerateDashboardUrlReturns("", serviceadapter.NewNotImplementedError("no dashboard!"))
			})

			It("returns the dashboard as blank", func() {
				Expect(serviceSpec.DashboardURL).To(BeEmpty())
			})

			It("does not error", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
			})

			It("still returns the bosh task ID and operation type", func() {
				var operationData broker.OperationData
				Expect(json.Unmarshal([]byte(serviceSpec.OperationData), &operationData)).To(Succeed())
				Expect(operationData).To(Equal(
					broker.OperationData{BoshTaskID: deployTaskID, OperationType: broker.OperationTypeCreate},
				))
			})
		})

		Context("adapter returns an error", func() {
			BeforeEach(func() {
				serviceAdapter.GenerateDashboardUrlReturns("", errors.New("fooo"))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("foo"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(provisionErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("includes the bosh task id", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("task-id: %d", deployTaskID),
					)))
				})
			})
		})

		Context("adapter returns an AdapterCommandError", func() {
			BeforeEach(func() {
				serviceAdapter.GenerateDashboardUrlReturns("",
					serviceadapter.NewUnknownFailureError(
						"it failed, but all is not lost dear user",
					),
				)
			})

			It("returns the user error", func() {
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
		})

		It("does not error", func() {
			Expect(provisionErr).NotTo(HaveOccurred())
		})

		It("returns the correct operation data", func() {
			var data broker.OperationData
			err := json.Unmarshal([]byte(serviceSpec.OperationData), &data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data.BoshContextID).NotTo(BeEmpty())
			Expect(data.Errands[0].Name).To(Equal(errandName))
			Expect(data.Errands[0].Instances).To(Equal([]string{errandInstance}))
			Expect(data.Errands[1].Name).To(Equal(errandName2))
			Expect(data.Errands[1].Instances).To(Equal([]string{errandInstance2}))
		})

		It("calls the deployer with a bosh context id", func() {
			Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
			_, _, _, actualBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
			Expect(actualBoshContextID).NotTo(BeEmpty())
		})

		Context("when provision is called again", func() {
			var (
				secondProvisionErr error
			)

			JustBeforeEach(func() {
				_, secondProvisionErr = b.Provision(
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
			})

			It("does not error", func() {
				Expect(secondProvisionErr).NotTo(HaveOccurred())
			})

			It("calls the deployer with a different bosh context id", func() {
				Expect(fakeDeployer.CreateCallCount()).To(Equal(2))
				_, _, _, firstBoshContextID, _ := fakeDeployer.CreateArgsForCall(0)
				Expect(firstBoshContextID).NotTo(BeNil())

				_, _, _, secondBoshContextID, _ := fakeDeployer.CreateArgsForCall(1)
				Expect(secondBoshContextID).NotTo(Equal(firstBoshContextID))
			})
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
		})

		It("does not error", func() {
			Expect(provisionErr).NotTo(HaveOccurred())
		})

		It("returns the correct operation data", func() {
			var data broker.OperationData
			err := json.Unmarshal([]byte(serviceSpec.OperationData), &data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data.BoshContextID).NotTo(BeEmpty())
		})
	})

	Context("when invalid json params are provided by the broker api", func() {
		BeforeEach(func() {
			jsonParams = []byte("not valid json")
		})

		It("wraps the returns a raw params invalid error", func() {
			Expect(provisionErr).To(Equal(brokerapi.ErrRawParamsInvalid))
		})
	})

	Context("when no arbitrary params are passed by user", func() {
		BeforeEach(func() {
			jsonParams = []byte{}
		})

		It("return no error", func() {
			Expect(provisionErr).NotTo(HaveOccurred())
		})

		It("no arbitrary params are passed to the adapter", func() {
			_, _, actualRequestParams, _, _ := fakeDeployer.CreateArgsForCall(0)
			Expect(actualRequestParams["parameters"]).To(HaveLen(0))
		})

		It("invokes the deployer", func() {
			Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
		})
	})

	Context("when a deployment has a generic error", func() {
		BeforeEach(func() {
			fakeDeployer.CreateReturns(0, []byte{}, fmt.Errorf("fooo"))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("error: fooo"))
		})

		Describe("returned error", func() {
			It("includes a standard message", func() {
				Expect(provisionErr).To(MatchError(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information:",
				)))
			})

			It("includes the broker request id", func() {
				Expect(provisionErr).To(MatchError(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				)))
			})

			It("includes the service name", func() {
				Expect(provisionErr).To(MatchError(ContainSubstring(
					"service: a-cool-redis-service",
				)))
			})

			It("includes the service instance guid", func() {
				Expect(provisionErr).To(MatchError(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				)))
			})

			It("includes the operation type", func() {
				Expect(provisionErr).To(MatchError(ContainSubstring(
					"operation: create",
				)))
			})

			It("does NOT include the bosh task id", func() {
				Expect(provisionErr).NotTo(MatchError(ContainSubstring(
					"task-id:",
				)))
			})
		})
	})

	Context("when getting the manifest has a bosh request error", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns([]byte{}, false, boshdirector.NewRequestError(
				fmt.Errorf("network timeout"),
			))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("error: could not get manifest: network timeout"))
		})

		It("returns the try again later error for the user", func() {
			Expect(provisionErr).To(MatchError(ContainSubstring("Currently unable to create service instance, please try again later")))
		})
	})

	Context("when a deploy has a bosh request error", func() {
		BeforeEach(func() {
			fakeDeployer.CreateReturns(0, []byte{}, boshdirector.NewRequestError(
				fmt.Errorf("error deploying instance: network timeout"),
			))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("error: error deploying instance: network timeout"))
		})

		It("returns the try again later error for the user", func() {
			Expect(provisionErr).To(MatchError(ContainSubstring("Currently unable to create service instance, please try again later")))
		})
	})

	Context("when a deployment has a user displayable error", func() {
		BeforeEach(func() {
			fakeDeployer.CreateReturns(0, []byte{}, broker.NewDisplayableError(fmt.Errorf("user message"), fmt.Errorf("operator message")))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("error: operator message"))
		})

		It("returns the error", func() {
			Expect(provisionErr).To(MatchError(ContainSubstring("user message")))
		})
	})

	Context("when the deploy returns an adapter error with a user message", func() {
		var err = serviceadapter.NewUnknownFailureError("it failed, but all is not lost dear user")

		BeforeEach(func() {
			fakeDeployer.CreateReturns(0, nil, err)
		})

		It("returns the user error", func() {
			Expect(provisionErr).To(Equal(err))
		})
	})

	Context("when the deploy returns an adapter error with no message", func() {
		var err = serviceadapter.NewUnknownFailureError("")

		BeforeEach(func() {
			fakeDeployer.CreateReturns(0, nil, err)
		})

		It("returns a generic error", func() {
			Expect(provisionErr).To(MatchError(ContainSubstring(
				"There was a problem completing your request. Please contact your operations team providing the following information:",
			)))
		})
	})

	Context("when a provision of an already provisioned instance is triggered", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns([]byte(`manifest: true`), true, nil)
		})

		It("returns an error", func() {
			Expect(provisionErr).To(Equal(brokerapi.ErrInstanceAlreadyExists))
		})
	})

	Context("when the async allowed flag is false", func() {
		BeforeEach(func() {
			asyncAllowed = false
		})
		It("returns a  error", func() {
			Expect(provisionErr).To(Equal(brokerapi.ErrAsyncRequired))
		})
	})

	Context("when count instances of plan fails", func() {
		BeforeEach(func() {
			cfClient.CountInstancesOfServiceOfferingReturns(nil, fmt.Errorf("count fail"))
		})

		Context("and no global quota is configured", func() {

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(provisionErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(provisionErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("count fail"))
			})

			It("makes no deployments", func() {
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})
		})

		Context("and a global quota is configured", func() {
			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(provisionErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(provisionErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("count fail"))
			})

			It("makes no deployments", func() {
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})
		})
	})

	Context("when the plan quota is reached", func() {
		BeforeEach(func() {
			planCounts := map[cf.ServicePlan]int{
				cfServicePlan("1234", existingPlanID, "url", "name"): serviceOfferingServiceInstanceLimit,
				cfServicePlan("1234", secondPlanID, "url", "name"):   0,
			}
			cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
		})

		It("fails", func() {
			By("returning an error")
			Expect(provisionErr).To(HaveOccurred())

			By("not deploying")
			Expect(fakeDeployer.CreateCallCount()).To(BeZero())
		})
	})

	Context("when the global quota is reached", func() {
		BeforeEach(func() {
			planCounts := map[cf.ServicePlan]int{
				cfServicePlan("1234", existingPlanID, "url", "name"): serviceOfferingServiceInstanceLimit,
				cfServicePlan("1234", secondPlanID, "url", "name"):   0,
			}
			cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
		})

		It("does not count the instances of each plan", func() {
			Expect(cfClient.CountInstancesOfPlanCallCount()).To(BeZero())
		})

		It("returns an error", func() {
			Expect(provisionErr).To(HaveOccurred())
		})

		It("makes no deployments", func() {
			Expect(fakeDeployer.CreateCallCount()).To(BeZero())
		})

		It("returns an error explaining the global quota has been reached", func() {
			Expect(provisionErr).To(MatchError(ContainSubstring(
				"The quota for this service has been exceeded. Please contact your Operator for help.",
			)))
		})

		It("logs on error for the operator explaining the global quota has been reached", func() {
			Expect(logBuffer.String()).To(
				ContainSubstring("service quota exceeded for service ID %s", serviceOfferingID),
			)
		})

		Context("and the instances of all plans cannot be counted", func() {
			BeforeEach(func() {
				cfClient.CountInstancesOfServiceOfferingReturns(nil, errors.New("count fail"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(provisionErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(provisionErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(provisionErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("count fail"))
			})

			It("makes no deployments", func() {
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})
		})
	})

	Describe("plan resource quotas", func() {
		When("a quota is set and has been reached", func() {
			JustBeforeEach(func() {
				plan := existingPlan

				plan.Quotas = config.Quotas{
					ResourceLimits: map[string]int{"ips": 10, "memory": 50},
				}
				plan.ResourceCosts = map[string]int{"ips": 5, "memory": 20}

				catalogWithResourceQuotas := serviceCatalog
				catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = nil
				catalogWithResourceQuotas.GlobalQuotas.ResourceLimits = nil
				catalogWithResourceQuotas.Plans = config.Plans{plan, secondPlan}
				fakeDeployer = new(brokerfakes.FakeDeployer)
				b = createBrokerWithServiceCatalog(catalogWithResourceQuotas)
			})

			BeforeEach(func() {
				planCounts := map[cf.ServicePlan]int{
					cfServicePlan("1234", existingPlanID, "url", "name"): 2,
				}
				cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
			})

			It("fails to provision when the quota is already reached", func() {
				serviceSpec, provisionErr = b.Provision(
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

				By("erroring")
				Expect(provisionErr).To(HaveOccurred())
				Expect(provisionErr.Error()).To(ContainSubstring(
					"plan quotas [ips (limit 10), memory (limit 50)] would be exceeded by this deployment",
				))

				By("not trying to deploy")
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})

			It("provisions successfully when the plan doesn't count against the plan quota", func() {
				serviceSpec, provisionErr = b.Provision(
					context.Background(),
					instanceID,
					brokerapi.ProvisionDetails{
						PlanID:           secondPlanID,
						RawContext:       jsonContext,
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

		Context("when no plan quota is set", func() {
			When("a provision of a plan with a resource cost is requested", func() {
				BeforeEach(func() {
					existingPlan.ResourceCosts["memory"] = 99
				})

				It("succeeds", func() {
					Expect(provisionErr).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("global resource quotas", func() {
		Context("when no quota is set", func() {
			When("a provision of a plan with a resource cost is requested", func() {
				BeforeEach(func() {
					existingPlan.ResourceCosts["memory"] = 99
				})

				It("succeeds", func() {
					Expect(provisionErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when a quota is set and has been reached", func() {
			When("a provision is requested", func() {
				JustBeforeEach(func() {
					plan := existingPlan

					plan.Quotas = config.Quotas{}
					plan.ResourceCosts = map[string]int{"ips": 1, "memory": 6}

					catalogWithResourceQuotas := serviceCatalog
					catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = nil
					catalogWithResourceQuotas.GlobalQuotas.ResourceLimits = map[string]int{"ips": 1, "memory": 10}
					catalogWithResourceQuotas.Plans = config.Plans{plan, secondPlan}
					fakeDeployer = new(brokerfakes.FakeDeployer)
					b = createBrokerWithServiceCatalog(catalogWithResourceQuotas)
				})

				BeforeEach(func() {
					planCounts := map[cf.ServicePlan]int{
						cfServicePlan("1234", existingPlanID, "url", "name"): 1,
					}
					cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
				})

				It("fails to provision when the quota is already reached", func() {
					serviceSpec, provisionErr = b.Provision(
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

					By("erroring")
					Expect(provisionErr).To(HaveOccurred())
					Expect(provisionErr.Error()).To(ContainSubstring("global quotas ["))
					Expect(provisionErr.Error()).To(ContainSubstring("ips (limit 1)"))
					Expect(provisionErr.Error()).To(ContainSubstring("memory (limit 10)"))
					Expect(provisionErr.Error()).To(ContainSubstring("] would be exceeded by this deployment"))

					By("not trying to deploy")
					Expect(fakeDeployer.CreateCallCount()).To(BeZero())
				})

				It("provisions successfully when the plan doesn't count against the global quota", func() {
					serviceSpec, provisionErr = b.Provision(
						context.Background(),
						instanceID,
						brokerapi.ProvisionDetails{
							PlanID:           secondPlanID,
							RawContext:       jsonContext,
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

	Context("when no quotas are specified", func() {
		BeforeEach(func() {
			planID = secondPlanID
		})

		It("succeeds", func() {
			Expect(provisionErr).NotTo(HaveOccurred())
		})

		It("makes a deployment", func() {
			Expect(fakeDeployer.CreateCallCount()).To(Equal(1))
		})

		It("doesn't count number of instaces", func() {
			Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(0))
		})

	})

	Context("when plan id given is not configured", func() {
		BeforeEach(func() {
			planID = "non-existant-pland"
		})

		It("return an error", func() {
			Expect(provisionErr).To(HaveOccurred())
		})
	})

	Context("adapter returns an error while fetching the dashboard url", func() {
		BeforeEach(func() {
			serviceAdapter.GenerateDashboardUrlReturns("", fmt.Errorf("no dashboard!"))
		})

		It("returns an error", func() {
			Expect(provisionErr).To(HaveOccurred())
		})
	})

	Context("when bosh can't be reached", func() {
		BeforeEach(func() {
			boshClient.GetInfoReturns(boshdirector.Info{}, errors.New("foo"))
		})

		It("returns an error", func() {
			Expect(provisionErr).To(HaveOccurred())
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
				Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
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

			It("requests the json schemas from the service adapter", func() {
				Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
			})

			It("succeeds", func() {
				Expect(provisionErr).NotTo(HaveOccurred())
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
})
