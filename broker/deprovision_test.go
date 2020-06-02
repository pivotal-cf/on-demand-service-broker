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
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("deprovisioning instances", func() {
	var (
		instanceID = "an-instance-to-be-deprovisioned"

		asyncAllowed    bool
		deprovisionSpec domain.DeprovisionServiceSpec
		deprovisionErr  error

		deleteTaskID       = 88
		deprovisionDetails domain.DeprovisionDetails
	)

	BeforeEach(func() {
		asyncAllowed = true
		boshClient.GetDeploymentReturns([]byte(`manifest: true`), true, nil)
		boshClient.DeleteDeploymentReturns(deleteTaskID, nil)
		deprovisionDetails = domain.DeprovisionDetails{PlanID: existingPlanID}
	})

	JustBeforeEach(func() {
		b = createDefaultBroker()
	})

	When("CF integration is disabled", func() {

		BeforeEach(func() {
			var err error
			b, err = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns that is deprovisioning asynchronously", func() {
			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionSpec.IsAsync).To(BeTrue())
		})

		It("logs that it run the pre-delete errand", func() {
			deprovisionDetails.PlanID = preDeleteErrandPlanID
			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
			))

		})

	})

	When("the async allowed flag is true", func() {
		It("succeeds deleting bosh deployment", func() {
			fakeUAAClient := new(fakes.FakeUAAClient)
			b.SetUAAClient(fakeUAAClient)

			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionSpec.IsAsync).To(BeTrue())

			By("validating OperationData")
			var operationData broker.OperationData
			Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				BoshTaskID:    deleteTaskID,
				OperationType: broker.OperationTypeDelete,
			}))

			By("validating DeleteDeployment args")
			Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
			actualInstanceID, _, force, _, _ := boshClient.DeleteDeploymentArgsForCall(0)
			Expect(actualInstanceID).To(Equal(deploymentName(instanceID)))
			Expect(force).To(BeFalse())

			By("validating logs")
			Expect(logBuffer.String()).To(SatisfyAll(
				MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} removing deployment for instance`),
				ContainSubstring(fmt.Sprintf("removing deployment for instance %s as part of operation \"delete\"", instanceID)),
				ContainSubstring(fmt.Sprintf("Bosh task id is %d for operation \"delete\" of instance %s", deleteTaskID, instanceID)),
				Not(ContainSubstring("pre-delete errand")),
			))

			By("deleting the service instance uaa client", func() {
				Expect(fakeUAAClient.DeleteClientCallCount()).To(Equal(1))
				actualClientID := fakeUAAClient.DeleteClientArgsForCall(0)
				Expect(actualClientID).To(Equal(instanceID))
			})
		})

		It("succeeds force deleting bosh deployment", func() {
			forceDeprovision := true
			deprovisionDetails.Force = forceDeprovision
			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionSpec.IsAsync).To(BeTrue())

			By("validating OperationData")
			var operationData broker.OperationData
			Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				BoshTaskID:    deleteTaskID,
				OperationType: broker.OperationTypeForceDelete,
			}))

			By("validating DeleteDeployment args")
			Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
			actualInstanceID, _, force, _, _ := boshClient.DeleteDeploymentArgsForCall(0)
			Expect(actualInstanceID).To(Equal(deploymentName(instanceID)))
			Expect(force).To(Equal(forceDeprovision))
			Expect(logBuffer.String()).To(SatisfyAll(
				ContainSubstring(fmt.Sprintf("removing deployment for instance %s as part of operation \"force-delete\"", instanceID)),
				ContainSubstring(fmt.Sprintf("Bosh task id is %d for operation \"force-delete\" of instance %s", deleteTaskID, instanceID)),
			))
		})
	})

	When("the async allowed flag is false", func() {
		BeforeEach(func() {
			asyncAllowed = false
		})

		It("returns an error", func() {
			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionErr).To(Equal(apiresponses.ErrAsyncRequired))
		})
	})

	Context("getting the deployment", func() {
		When("it returns a request error", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, boshdirector.NewRequestError(errors.New("problem fetching manifest")))
			})

			It("returns an error and logs it", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(HaveOccurred())
				Expect(logBuffer.String()).To(
					ContainSubstring("error: problem fetching manifest. error for user: Currently unable to delete service instance, please try again later."),
				)
			})
		})

		When("it returns a non-request error", func() {
			err := errors.New("oops")

			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, err)
			})

			It("returns an error and logs it", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(HaveOccurred())
				Expect(logBuffer.String()).To(
					ContainSubstring(fmt.Sprintf("error deprovisioning: cannot get deployment %s: %s", deploymentName(instanceID), err)),
				)
			})
		})

		When("it returns that deployment is not found", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, nil)
			})

			Context("and disable_bosh_configs is true", func() {
				BeforeEach(func() {
					brokerConfig.DisableBoshConfigs = true
				})

				It("doesn't call DeleteConfigs", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(boshClient.DeleteConfigsCallCount()).To(Equal(0), "DeleteConfigs was called")
				})
			})

			Context("bosh configs can be deleted", func() {
				It("deletes the bosh configs and returns expected error about missing deployment", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(boshClient.DeleteConfigsCallCount()).To(Equal(1))
					Expect(deprovisionErr).To(Equal(apiresponses.ErrInstanceDoesNotExist))
					Expect(logBuffer.String()).To(ContainSubstring(
						fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID),
					))
				})
			})

			Context("deleting bosh configs fails", func() {
				BeforeEach(func() {
					boshClient.DeleteConfigsReturns(errors.New("oops"))
				})

				It("returns error about deleting service", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(deprovisionErr).To(MatchError("Unable to delete service. Please try again later or contact your operator."))
					Expect(logBuffer.String()).To(ContainSubstring(
						fmt.Sprintf("error deprovisioning: failed to delete configs for instance service-instance_%s", instanceID),
					))
				})
			})

			Context("and removing secrets succeeds", func() {
				BeforeEach(func() {
					fakeSecretManager.DeleteSecretsForInstanceReturns(nil)
				})

				It("returns an error and logs it", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(deprovisionErr).To(Equal(apiresponses.ErrInstanceDoesNotExist))
					Expect(logBuffer.String()).To(ContainSubstring(
						fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID),
					))
				})
			})

			Context("and removing secrets fails", func() {
				BeforeEach(func() {
					fakeSecretManager.DeleteSecretsForInstanceReturns(errors.New("oops"))
				})

				It("returns an error and logs it", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(deprovisionErr).To(MatchError("Unable to delete service. Please try again later or contact your operator."))
					Expect(logBuffer.String()).To(ContainSubstring(
						fmt.Sprintf("error deprovisioning: failed to delete secrets for instance service-instance_%s", instanceID),
					))
				})
			})
		})
	})

	When("the deployment has a pre-delete errand", func() {
		errandTaskID := 123

		BeforeEach(func() {
			instanceID = "an-instance-with-pre-delete-errand"
			deprovisionDetails.PlanID = preDeleteErrandPlanID
			boshClient.RunErrandReturns(errandTaskID, nil)
		})

		It("returns that is deprovisioning asynchronously", func() {
			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionSpec.IsAsync).To(BeTrue())

			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
			))

			By("executing errand")
			Expect(boshClient.RunErrandCallCount()).To(Equal(1))
			argDeploymentName, argErrandName, _, contextID, _, _ := boshClient.RunErrandArgsForCall(0)
			Expect(argDeploymentName).To(Equal(broker.InstancePrefix + instanceID))
			Expect(argErrandName).To(Equal("cleanup-resources"))
			Expect(contextID).To(MatchRegexp(
				`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			))

			Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(0))

			var operationData broker.OperationData
			_, _, _, contextID, _, _ = boshClient.RunErrandArgsForCall(0)

			Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				BoshTaskID:    errandTaskID,
				BoshContextID: contextID,
				OperationType: broker.OperationTypeDelete,
				Errands:       []config.Errand{{Name: "cleanup-resources", Instances: []string{}}},
			}))
		})

		Context("and the errand is colocated", func() {
			var errandName, planID, errandInstance string

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
				deprovisionDetails.PlanID = planID
				serviceCatalog.Plans = config.Plans{preDeleteErrandPlan}
			})

			It("returns the correct operation data", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				var data broker.OperationData
				_, errandName, _, contextID, _, _ := boshClient.RunErrandArgsForCall(0)
				Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &data)).To(Succeed())
				Expect(data.BoshContextID).To(Equal(contextID))
				Expect(data.OperationType).To(Equal(broker.OperationTypeDelete))
				Expect(data.Errands[0].Name).To(Equal(errandName))
				Expect(data.Errands[0].Instances).To(Equal([]string{errandInstance}))
				Expect(data.Errands).To(Equal([]config.Errand{{Name: "cleanup-errand", Instances: []string{errandInstance}}}))
			})
		})

		When("bosh returns an error attempting to run errand", func() {
			BeforeEach(func() {
				boshClient.RunErrandReturns(0, errors.New("something went wrong"))
			})

			It("returns the error", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(HaveOccurred())
			})
		})

		When("force-delete is passed", func() {
			It("returns force-delete operation data", func() {
				deprovisionDetails.Force = true
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
				))

				Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(0))

				var operationData broker.OperationData
				_, _, _, contextID, _, _ := boshClient.RunErrandArgsForCall(0)

				Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
				Expect(operationData).To(Equal(broker.OperationData{
					BoshTaskID:    errandTaskID,
					BoshContextID: contextID,
					OperationType: broker.OperationTypeForceDelete,
					Errands:       []config.Errand{{Name: "cleanup-resources", Instances: []string{}}},
				}))
			})
		})
	})

	When("deleting a deployment fails", func() {
		Context("with a generic error", func() {
			BeforeEach(func() {
				boshClient.DeleteDeploymentReturns(0, errors.New("er ma gerd!"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					deprovisionSpec, deprovisionErr = b.Deprovision(
						context.Background(),
						instanceID,
						deprovisionDetails,
						asyncAllowed,
					)

					Expect(deprovisionErr).To(MatchError(ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:")))
					Expect(deprovisionErr).To(MatchError(MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)))
					Expect(deprovisionErr).To(MatchError(ContainSubstring("service: a-cool-redis-service")))
					Expect(deprovisionErr).To(MatchError(ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID))))
					Expect(deprovisionErr).To(MatchError(ContainSubstring("operation: delete")))
					Expect(deprovisionErr).NotTo(MatchError(ContainSubstring("task-id:")))

					Expect(logBuffer.String()).To(ContainSubstring("er ma gerd!"))
				})
			})
		})

		Context("with a bosh request error", func() {
			BeforeEach(func() {
				boshClient.DeleteDeploymentReturns(0, boshdirector.NewRequestError(
					fmt.Errorf("error deleting instance: network timeout"),
				))
			})

			It("logs and returns the error", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(logBuffer.String()).To(ContainSubstring("error deleting instance: network timeout"))
				Expect(deprovisionErr).To(MatchError(ContainSubstring("Currently unable to delete service instance, please try again later")))
			})
		})
	})

	Context("getting bosh tasks in progress", func() {
		When("a task is in progress", func() {
			incompleteTasks := boshdirector.BoshTasks{{ID: 1337, State: boshdirector.TaskProcessing}}
			BeforeEach(func() {
				boshClient.GetTasksInProgressReturns(incompleteTasks, nil)
			})

			It("logs and returns an error", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(MatchError("An operation is in progress for your service instance. Please try again later."))
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error deprovisioning: deployment %s is still in progress: tasks %s\n", deploymentName(instanceID), incompleteTasks.ToLog())))
			})
		})

		Context("request error", func() {
			BeforeEach(func() {
				boshClient.GetTasksInProgressReturns(
					boshdirector.BoshTasks{},
					boshdirector.NewRequestError(errors.New("problem fetching tasks")),
				)
			})

			It("logs and returns an error", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(HaveOccurred())
				Expect(logBuffer.String()).To(
					ContainSubstring("error: problem fetching tasks. error for user: Currently unable to delete service instance, please try again later."),
				)
			})
		})

		Context("non-request error", func() {
			BeforeEach(func() {
				boshClient.GetTasksInProgressReturns(boshdirector.BoshTasks{}, errors.New("oops"))
			})

			It("returns an error", func() {
				deprovisionSpec, deprovisionErr = b.Deprovision(
					context.Background(),
					instanceID,
					deprovisionDetails,
					asyncAllowed,
				)

				Expect(deprovisionErr).To(HaveOccurred())
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error deprovisioning: cannot get tasks for deployment %s", deploymentName(instanceID))))
			})
		})
	})

	When("deleting the uaa client fails", func() {
		It("should not fail the operation", func() {
			fakeUAAClient := new(fakes.FakeUAAClient)
			b.SetUAAClient(fakeUAAClient)

			fakeUAAClient.DeleteClientReturns(errors.New("failed to delete!"))

			deprovisionSpec, deprovisionErr = b.Deprovision(
				context.Background(),
				instanceID,
				deprovisionDetails,
				asyncAllowed,
			)

			Expect(deprovisionErr).NotTo(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("failed to delete UAA client associated with service instance %s", instanceID),
			))
		})

	})
})
