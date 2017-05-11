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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

var _ = Describe("deprovisioning instances", func() {
	var (
		instanceID = "an-instance-to-be-deprovisioned"

		asyncAllowed    bool
		deprovisionSpec brokerapi.DeprovisionServiceSpec
		deprovisionErr  error

		deleteTaskID = 88
	)

	BeforeEach(func() {
		asyncAllowed = true
		boshClient.GetDeploymentReturns([]byte(`manifest: true`), true, nil)
		cfClient.GetInstanceStateReturns(cf.InstanceState{
			PlanID:              existingPlanID,
			OperationInProgress: false,
		}, nil)
		boshClient.DeleteDeploymentReturns(deleteTaskID, nil)
	})

	JustBeforeEach(func() {
		deprovisionSpec, deprovisionErr = b.Deprovision(
			context.Background(),
			instanceID,
			brokerapi.DeprovisionDetails{},
			asyncAllowed,
		)
	})

	It("returns that is deprovisioning asynchronously", func() {
		Expect(deprovisionSpec.IsAsync).To(BeTrue())
	})

	It("returns the bosh task ID and operation type in the operation data", func() {
		var operationData broker.OperationData
		Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
		Expect(operationData).To(Equal(broker.OperationData{
			BoshTaskID:    deleteTaskID,
			OperationType: broker.OperationTypeDelete,
		}))
	})

	It("deletes a bosh deployment whose name is based on the instance ID", func() {
		Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
		actualInstanceID, _, _ := boshClient.DeleteDeploymentArgsForCall(0)
		Expect(actualInstanceID).To(Equal(deploymentName(instanceID)))
	})

	It("logs that it will delete the deployment with a request ID", func() {
		Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} deleting deployment for instance`))
	})

	It("logs the task id to delete the deployment", func() {
		Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("Bosh task id for Delete instance %s was %d", instanceID, deleteTaskID)))
	})

	It("does not log anything about pre-delete errands", func() {
		Expect(logBuffer.String()).NotTo(ContainSubstring("pre-delete errand"))
	})

	Context("when the async allowed flag is false", func() {
		BeforeEach(func() {
			asyncAllowed = false
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(Equal(brokerapi.ErrAsyncRequired))
		})
	})

	Context("when getting the deployment returns a request error", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns(nil, false, boshdirector.NewRequestError(errors.New("problem fetching manifest")))
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(HaveOccurred())
		})

		It("logs an error", func() {
			Expect(logBuffer.String()).To(
				ContainSubstring("error: problem fetching manifest. error for user: Currently unable to delete service instance, please try again later."),
			)
		})
	})

	Context("when getting the deployment returns a non-request error", func() {
		err := errors.New("oops")

		BeforeEach(func() {
			boshClient.GetDeploymentReturns(nil, false, err)
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(HaveOccurred())
		})

		It("logs an error", func() {
			Expect(logBuffer.String()).To(
				ContainSubstring(fmt.Sprintf("error deprovisioning: cannot get deployment %s: %s", deploymentName(instanceID), err)),
			)
		})
	})

	Context("when getting the deployment returns that deployment is not found", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns(nil, false, nil)
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(Equal(brokerapi.ErrInstanceDoesNotExist))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID),
			))
		})
	})

	Context("when the deployment has a pre-delete errand", func() {
		errandTaskID := 123

		BeforeEach(func() {
			instanceID = "an-instance-with-pre-delete-errand"
			cfClient.GetInstanceStateReturns(cf.InstanceState{
				PlanID:              preDeleteErrandPlanID,
				OperationInProgress: false,
			}, nil)
			boshClient.RunErrandReturns(errandTaskID, nil)
		})

		It("returns that is deprovisioning asynchronously", func() {
			Expect(deprovisionSpec.IsAsync).To(BeTrue())
		})

		It("calls get instance state with expected instance ID", func() {
			Expect(cfClient.GetInstanceStateCallCount()).To(Equal(1))
			argInstanceID, _ := cfClient.GetInstanceStateArgsForCall(0)
			Expect(argInstanceID).To(Equal(instanceID))
		})

		It("logs that it run the pre-delete errand", func() {
			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
			))
		})

		It("executes the specified errand", func() {
			Expect(boshClient.RunErrandCallCount()).To(Equal(1))
			argDeploymentName, argErrandName, contextID, _ := boshClient.RunErrandArgsForCall(0)
			Expect(argDeploymentName).To(Equal(broker.InstancePrefix + instanceID))
			Expect(argErrandName).To(Equal("cleanup-resources"))
			Expect(contextID).To(MatchRegexp(
				`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			))
		})

		It("does not call delete deployment", func() {
			Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(0))
		})

		It("includes the operation type, task id, and context id in the operation data", func() {
			var operationData broker.OperationData

			_, _, contextID, _ := boshClient.RunErrandArgsForCall(0)

			Expect(json.Unmarshal([]byte(deprovisionSpec.OperationData), &operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				BoshTaskID:    errandTaskID,
				BoshContextID: contextID,
				OperationType: broker.OperationTypeDelete,
			}))
		})

		Context("when the cf client returns an error from get instance state", func() {
			BeforeEach(func() {
				cfClient.GetInstanceStateReturns(
					cf.InstanceState{},
					errors.New("service instance error"),
				)
			})

			It("returns an error", func() {
				Expect(deprovisionErr).To(HaveOccurred())
			})
		})

		Context("when bosh returns an error attempting to run errand", func() {
			BeforeEach(func() {
				boshClient.RunErrandReturns(0, errors.New("something went wrong"))
			})

			It("returns the error", func() {
				Expect(deprovisionErr).To(HaveOccurred())
			})
		})
	})

	Describe("when deleting a deployment fails", func() {
		Context("with a generic error", func() {
			BeforeEach(func() {
				boshClient.DeleteDeploymentReturns(0, errors.New("er ma gerd!"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(deprovisionErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(deprovisionErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(deprovisionErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(deprovisionErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(deprovisionErr).To(MatchError(ContainSubstring(
						"operation: delete",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(deprovisionErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("er ma gerd!"))
			})
		})

		Context("with a bosh request error", func() {
			BeforeEach(func() {
				boshClient.DeleteDeploymentReturns(0, boshdirector.NewRequestError(
					fmt.Errorf("error deleting instance: network timeout"),
				))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("error deleting instance: network timeout"))
			})

			It("returns the try again later error for the user", func() {
				Expect(deprovisionErr).To(MatchError(ContainSubstring("Currently unable to delete service instance, please try again later")))
			})
		})
	})

	Context("when a bosh task is in flight for the deployment", func() {
		incompleteTasks := boshdirector.BoshTasks{{ID: 1337, State: boshdirector.TaskProcessing}}
		BeforeEach(func() {
			boshClient.GetTasksReturns(incompleteTasks, nil)
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(MatchError("An operation is in progress for your service instance. Please try again later."))
		})

		It("logs an error", func() {
			Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error deprovisioning: deployment %s is still in progress: tasks %s\n", deploymentName(instanceID), incompleteTasks.ToLog())))
		})
	})

	Context("when getting bosh tasks returns a request error", func() {
		BeforeEach(func() {
			boshClient.GetTasksReturns(
				boshdirector.BoshTasks{},
				boshdirector.NewRequestError(errors.New("problem fetching tasks")),
			)
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(HaveOccurred())
		})

		It("logs an error", func() {
			Expect(logBuffer.String()).To(
				ContainSubstring("error: problem fetching tasks. error for user: Currently unable to delete service instance, please try again later."),
			)
		})
	})

	Context("when getting bosh tasks returns a non-request error", func() {
		BeforeEach(func() {
			boshClient.GetTasksReturns(boshdirector.BoshTasks{}, errors.New("oops"))
		})

		It("returns an error", func() {
			Expect(deprovisionErr).To(HaveOccurred())
		})

		It("logs an error", func() {
			Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error deprovisioning: cannot get tasks for deployment %s", deploymentName(instanceID))))
		})
	})
})
