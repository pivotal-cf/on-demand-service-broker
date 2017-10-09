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
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("provisioning", func() {
	var (
		planID         string
		errandName     string
		errandInstance string

		serviceSpec  brokerapi.ProvisionedServiceSpec
		provisionErr error

		organizationGUID = "a-cf-org"
		spaceGUID        = "a-cf-space"
		instanceID       = "some-instance-id"
		jsonParams       []byte
		arbParams        = map[string]interface{}{"foo": "bar"}

		asyncAllowed = true
	)

	BeforeEach(func() {
		planID = existingPlanID
		asyncAllowed = true
		var err error
		jsonParams, err = json.Marshal(arbParams)
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

	Context("when the plan has a post-deploy lifecycle errand", func() {
		BeforeEach(func() {
			planID = "post-deploy-errand-plan-id"
			errandName = "health-check"
			errandInstance = "post-deploy-instance-group-name/0"

			postDeployErrandPlan := config.Plan{
				ID: planID,
				LifecycleErrands: &config.LifecycleErrands{
					PostDeploy:          errandName,
					PostDeployInstances: []string{errandInstance},
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
			Expect(data.PostDeployErrandName).To(Equal(errandName))
			Expect(data.PostDeployErrandInstances).To(Equal([]string{errandInstance}))
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
			Expect(actualRequestParams["parameters"]).To(BeNil())
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
			cfClient.CountInstancesOfPlanReturns(0, fmt.Errorf("count fail"))
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
			cfClient.CountInstancesOfPlanReturns(existingPlanServiceInstanceLimit, nil)
		})

		Context("and no global quota is set", func() {

			It("counts the instances for the right plan", func() {
				Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(1))
				actualServiceID, actualPlanID, _ := cfClient.CountInstancesOfPlanArgsForCall(0)

				Expect(actualServiceID).To(Equal(serviceOfferingID))
				Expect(actualPlanID).To(Equal(planID))
			})

			It("returns an error", func() {
				Expect(provisionErr).To(HaveOccurred())
			})

			It("makes no deployments", func() {
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})
		})

		Context("and the global quota is set", func() {
			It("counts the instances of each plan", func() {
				Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(provisionErr).To(HaveOccurred())
			})

			It("makes no deployments", func() {
				Expect(fakeDeployer.CreateCallCount()).To(BeZero())
			})
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

		It("counts the instances of all plans", func() {
			Expect(cfClient.CountInstancesOfServiceOfferingCallCount()).To(Equal(2))
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
				callCount := 0
				cfClient.CountInstancesOfServiceOfferingStub = func(_ string, _ *log.Logger) (map[cf.ServicePlan]int, error) {
					callCount++
					if callCount > 1 {
						return nil, errors.New("count fail")
					}
					return nil, nil
				}
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
})
