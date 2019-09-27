// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package deleter_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/deleter/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

var _ = Describe("Deleter", func() {
	const (
		serviceUniqueID              = "some-unique-service-id"
		serviceInstance1GUID         = "service-instance-1-guid"
		serviceInstance2GUID         = "service-instance-2-guid"
		serviceInstance1BoundAppGUID = "service-instance-1-bound-app-guid"
		serviceInstance1BindingGUID  = "service-instance-1-binding-guid"
		serviceInstance1KeyGUID      = "service-instance-1-key-guid"
		pollingInitialOffset         = 10
		pollingInterval              = 5
	)

	var (
		deleteTool *deleter.Deleter
		cfClient   *fakes.FakeCloudFoundryClient
		sleeper    *fakes.FakeSleeper
		logger     *log.Logger
		logBuffer  *bytes.Buffer

		serviceKey cf.ServiceKey
		binding    cf.Binding
	)

	BeforeEach(func() {
		logBuffer = new(bytes.Buffer)
		logger = loggerfactory.
			New(io.MultiWriter(GinkgoWriter, logBuffer), "[deleter-unit-tests] ", log.LstdFlags).
			NewWithRequestID()

		cfClient = new(fakes.FakeCloudFoundryClient)

		cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)

		binding = cf.Binding{
			GUID:    serviceInstance1BindingGUID,
			AppGUID: serviceInstance1BoundAppGUID,
		}
		cfClient.GetBindingsForInstanceReturns([]cf.Binding{binding}, nil)

		serviceKey = cf.ServiceKey{
			GUID: serviceInstance1KeyGUID,
		}
		cfClient.GetServiceKeysForInstanceReturns([]cf.ServiceKey{serviceKey}, nil)

		notFoundError := cf.NewResourceNotFoundError("service instance not found")
		cfClient.GetLastOperationForInstanceReturns(cf.LastOperation{}, notFoundError)

		sleeper = new(fakes.FakeSleeper)
		deleteTool = deleter.New(cfClient, sleeper, pollingInitialOffset, pollingInterval, logger)
	})

	It("logs its polling configuration at startup", func() {
		deleteTool.DeleteAllServiceInstances(serviceUniqueID)
		Expect(logBuffer.String()).To(ContainSubstring("Deleter Configuration: polling_intial_offset: %d, polling_interval: %d.", pollingInitialOffset, pollingInterval))
	})

	Context("when no service instances exist", func() {
		It("logs that there are no instances", func() {
			cfClient.GetServiceInstancesReturns([]cf.Instance{}, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).NotTo(HaveOccurred())

			Expect(logBuffer.String()).To(ContainSubstring("No service instances found."))

			Expect(cfClient.GetServiceInstancesCallCount()).To(Equal(1), "Expected to call get instances only once")
		})
	})

	Context("when it succeeds", func() {
		Context("when two service instances are deleted immediately", func() {
			BeforeEach(func() {
				cfClient.GetServiceInstancesReturnsOnCall(0, []cf.Instance{
					{GUID: serviceInstance1GUID},
					{GUID: serviceInstance2GUID},
				}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)
				cfClient.GetBindingsForInstanceReturnsOnCall(0, []cf.Binding{binding}, nil)
				cfClient.GetBindingsForInstanceReturnsOnCall(1, []cf.Binding{}, nil)
				cfClient.GetServiceKeysForInstanceReturnsOnCall(0, []cf.ServiceKey{serviceKey}, nil)
				cfClient.GetServiceKeysForInstanceReturnsOnCall(1, []cf.ServiceKey{}, nil)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("by deleting instances", func() {
				By("By getting all instances for the service offering")

				actualServiceUniqueID, actualLogger := cfClient.GetServiceInstancesArgsForCall(0)
				Expect(actualServiceUniqueID.ServiceOfferingID).To(Equal(serviceUniqueID))
				Expect(actualLogger).To(Equal(logger))

				By("Deleting service instance 1")

				// Get bindings for service instance 1
				actualInstanceGUID, _ := cfClient.GetBindingsForInstanceArgsForCall(0)
				Expect(actualInstanceGUID).To(Equal(serviceInstance1GUID))

				// Delete app binding for service instance 1
				Expect(logBuffer.String()).To(ContainSubstring("Deleting binding %s of service instance %s to app %s", binding.GUID, serviceInstance1GUID, binding.AppGUID))
				actualBinding, _ := cfClient.DeleteBindingArgsForCall(0)
				Expect(actualBinding.GUID).To(Equal(serviceInstance1BindingGUID))
				Expect(actualBinding.AppGUID).To(Equal(serviceInstance1BoundAppGUID))

				// Get service keys for service instance 1
				actualInstanceGUID, _ = cfClient.GetServiceKeysForInstanceArgsForCall(0)
				Expect(actualInstanceGUID).To(Equal(serviceInstance1GUID))

				// Delete service key for service instance 1
				Expect(logBuffer.String()).To(ContainSubstring("Deleting service key %s of service instance %s", serviceKey.GUID, serviceInstance1GUID))
				actualServiceKey, _ := cfClient.DeleteServiceKeyArgsForCall(0)
				Expect(actualServiceKey.GUID).To(Equal(serviceInstance1KeyGUID))

				// Delete service instance 1
				Expect(logBuffer.String()).To(ContainSubstring("Deleting service instance %s", serviceInstance1GUID))
				actualInstanceGUID, _ = cfClient.DeleteServiceInstanceArgsForCall(0)
				Expect(actualInstanceGUID).To(Equal(serviceInstance1GUID))

				// Get instance 1
				Expect(logBuffer.String()).To(ContainSubstring("Waiting for service instance %s to be deleted", serviceInstance1GUID))
				actualInstanceGUID, _ = cfClient.GetLastOperationForInstanceArgsForCall(1)
				Expect(actualInstanceGUID).To(Equal(serviceInstance1GUID))

				By("Deleting service instance 2")

				// Get bindings for service instance 2
				actualInstanceGUID, _ = cfClient.GetBindingsForInstanceArgsForCall(1)
				Expect(actualInstanceGUID).To(Equal(serviceInstance2GUID))

				// No bindings to delete for service instance 2
				Expect(cfClient.DeleteBindingCallCount()).To(Equal(1), "expected to call delete bindings only once")

				// Get service keys for service instance 2
				actualInstanceGUID, _ = cfClient.GetServiceKeysForInstanceArgsForCall(1)
				Expect(actualInstanceGUID).To(Equal(serviceInstance2GUID))

				// No service keys to delete for service instance 2
				Expect(cfClient.DeleteServiceKeyCallCount()).To(Equal(1), "expected to call delete service keys only once")

				// Delete service instance 2
				Expect(logBuffer.String()).To(ContainSubstring("Deleting service instance %s", serviceInstance2GUID))
				actualInstanceGUID, _ = cfClient.DeleteServiceInstanceArgsForCall(1)
				Expect(actualInstanceGUID).To(Equal(serviceInstance2GUID))

				// Get instance 2
				Expect(logBuffer.String()).To(ContainSubstring("Waiting for service instance %s to be deleted", serviceInstance2GUID))
				actualInstanceGUID, _ = cfClient.GetLastOperationForInstanceArgsForCall(2)
				Expect(actualInstanceGUID).To(Equal(serviceInstance2GUID))

				By("Verifying that all service instances have been deleted")

				actualServiceUniqueID, actualLogger = cfClient.GetServiceInstancesArgsForCall(1)
				Expect(actualServiceUniqueID.ServiceOfferingID).To(Equal(serviceUniqueID))
				Expect(actualLogger).To(Equal(logger))
			})
		})

		Context("when the service instance is not deleted immediately", func() {
			BeforeEach(func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				cfClient.GetLastOperationForInstanceReturnsOnCall(0, cf.LastOperation{
					Type:  "any-non-delete-status",
					State: cf.OperationState("success"),
				}, nil)

				lastOperation := cf.LastOperation{
					Type:  cf.OperationTypeDelete,
					State: cf.OperationState("in progress"),
				}
				cfClient.GetLastOperationForInstanceReturnsOnCall(1, lastOperation, nil)
				notFoundError := cf.NewResourceNotFoundError("service lastOperation not found")
				cfClient.GetLastOperationForInstanceReturnsOnCall(2, cf.LastOperation{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the instance", func() {
				Expect(cfClient.GetLastOperationForInstanceCallCount()).To(Equal(3), "Expected to get instance three times")
				Expect(logBuffer.String()).To(ContainSubstring("Result: deleted service instance %s", serviceInstance1GUID))
			})

			It("waits before starting last operation requests", func() {
				Expect(sleeper.SleepCallCount()).To(Equal(3))
				Expect(sleeper.SleepArgsForCall(0)).To(Equal(pollingInitialOffset * time.Second))
			})

			It("waits in between last operation requests", func() {
				Expect(sleeper.SleepCallCount()).To(Equal(3))
				Expect(sleeper.SleepArgsForCall(1)).To(Equal(pollingInterval * time.Second))
				Expect(sleeper.SleepArgsForCall(2)).To(Equal(pollingInterval * time.Second))
			})
		})

		Context("when the service instance operation is delete succeeded", func() {
			BeforeEach(func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				lastOperation := cf.LastOperation{
					Type:  cf.OperationTypeDelete,
					State: cf.OperationState("succeeded"),
				}
				cfClient.GetLastOperationForInstanceReturnsOnCall(0, lastOperation, nil)
				notFoundError := cf.NewResourceNotFoundError("service instance not found")
				cfClient.GetLastOperationForInstanceReturnsOnCall(1, cf.LastOperation{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("by deleting instances", func() {
				Expect(cfClient.GetLastOperationForInstanceCallCount()).To(Equal(2), "Expected to get instance two times")

				Expect(logBuffer.String()).To(ContainSubstring("Result: deleted service instance %s", serviceInstance1GUID))
			})
		})

		When("the service instance is already been deleted", func() {
			BeforeEach(func() {
				cfClient.DeleteServiceInstanceReturns(errors.New("Operation in progress"))

				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				lastOperation := cf.LastOperation{
					Type:  cf.OperationTypeDelete,
					State: cf.OperationState("in progress"),
				}
				cfClient.GetLastOperationForInstanceReturnsOnCall(0, lastOperation, nil)
				cfClient.GetLastOperationForInstanceReturnsOnCall(1, lastOperation, nil)
				notFoundError := cf.NewResourceNotFoundError("service instanceBeingDeleted not found")
				cfClient.GetLastOperationForInstanceReturnsOnCall(2, cf.LastOperation{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't try to delete the instance", func() {
				Expect(cfClient.DeleteServiceInstanceCallCount()).To(BeZero())
				Expect(logBuffer.String()).To(ContainSubstring("service instance %s is being deleted", serviceInstance1GUID))
			})

			It("waits while the instance is being deleted", func() {
				Expect(logBuffer.String()).To(ContainSubstring("Waiting for service instance %s to be deleted", serviceInstance1GUID))

				Expect(sleeper.SleepCallCount()).To(Equal(3))
				Expect(sleeper.SleepArgsForCall(1)).To(Equal(pollingInterval * time.Second))
				Expect(sleeper.SleepArgsForCall(2)).To(Equal(pollingInterval * time.Second))

				Expect(cfClient.GetLastOperationForInstanceCallCount()).To(Equal(3))
			})
		})

		When("It cannot retrieve instance latest operation information", func() {
			BeforeEach(func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				cfClient.GetLastOperationForInstanceReturnsOnCall(0, cf.LastOperation{}, errors.New("failed to get instance"))
				cfClient.GetLastOperationForInstanceReturnsOnCall(1, cf.LastOperation{}, cf.NewResourceNotFoundError("service instanceBeingDeleted not found"))

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("logs a warning and deletes the instance", func() {
				Expect(logBuffer.String()).To(ContainSubstring("could not retrieve information about service instance %s", serviceInstance1GUID))
				Expect(cfClient.DeleteServiceInstanceCallCount()).To(Equal(1))
			})
		})

		Context("when get bindings returns a resource not found error", func() {
			It("skips deleting the binding and continues", func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				cfClient.GetBindingsForInstanceReturns(
					[]cf.Binding{},
					cf.NewResourceNotFoundError("no instance!"),
				)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfClient.DeleteBindingCallCount()).To(Equal(0))
			})
		})

		Context("when get service keys returns a resource not found error", func() {
			It("skips deleting the binding and continues", func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				cfClient.GetServiceKeysForInstanceReturns(
					[]cf.ServiceKey{},
					cf.NewResourceNotFoundError("no instance!"),
				)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfClient.DeleteServiceKeyCallCount()).To(Equal(0))
			})
		})

		Context("when the get service instance request fails", func() {
			BeforeEach(func() {
				cfClient.GetServiceInstancesReturns([]cf.Instance{{GUID: serviceInstance1GUID}}, nil)
				cfClient.GetServiceInstancesReturnsOnCall(1, []cf.Instance{}, nil)

				cfClient.GetLastOperationForInstanceReturnsOnCall(0, cf.LastOperation{}, errors.New("request failed"))
				notFoundError := cf.NewResourceNotFoundError("service instance not found")
				cfClient.GetLastOperationForInstanceReturnsOnCall(1, cf.LastOperation{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("continues polling", func() {
				Expect(cfClient.GetLastOperationForInstanceCallCount()).To(Equal(2), "Expected to get instance two times")

				Expect(logBuffer.String()).To(ContainSubstring("Result: deleted service instance %s", serviceInstance1GUID))
			})
		})
	})

	Context("when get all service instances returns an error", func() {
		It("returns an error", func() {
			cfClient.GetServiceInstancesReturns([]cf.Instance{}, errors.New("cannot get instances"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot get instances"))
		})
	})

	Context("when get bindings returns a non-ResourceNotFound error", func() {
		It("returns an error", func() {
			cfClient.GetBindingsForInstanceReturns([]cf.Binding{}, errors.New("error getting bindings"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("error getting bindings"))
		})
	})

	Context("when delete binding returns an error", func() {
		It("returns an error", func() {
			cfClient.DeleteBindingReturns(errors.New("error deleting binding"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("error deleting binding"))
		})
	})

	Context("when get service keys returns an error", func() {
		It("returns an error", func() {
			cfClient.GetServiceKeysForInstanceReturns([]cf.ServiceKey{}, errors.New("error getting service keys"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("error getting service keys"))
		})
	})

	Context("when delete service key returns an error", func() {
		It("returns an error", func() {
			cfClient.DeleteServiceKeyReturns(errors.New("error deleting service key"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("error deleting service key"))
		})
	})

	Context("when delete service instance returns an error", func() {
		It("returns an error", func() {
			cfClient.DeleteServiceInstanceReturns(errors.New("error deleting service instance"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("error deleting service instance"))
		})
	})

	Context("when get service instance returns delete failed", func() {
		It("returns an error", func() {
			lastOperation := cf.LastOperation{
				Type:  cf.OperationTypeDelete,
				State: cf.OperationStateFailed,
			}
			cfClient.GetLastOperationForInstanceReturns(lastOperation, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Delete operation failed.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns the wrong last operation type", func() {
		It("returns an error", func() {
			lastOperation := cf.LastOperation{
				Type:  cf.OperationType("update"),
				State: cf.OperationStateFailed,
			}
			cfClient.GetLastOperationForInstanceReturns(lastOperation, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Unexpected operation type: 'update'.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns an unauthorized error", func() {
		It("returns an error", func() {
			cfClient.GetLastOperationForInstanceReturns(cf.LastOperation{}, cf.NewUnauthorizedError("not logged in"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not logged in.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns a forbidden error", func() {
		It("returns an error", func() {
			cfClient.GetLastOperationForInstanceReturns(cf.LastOperation{}, cf.NewForbiddenError("not permitted"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not permitted.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns an invalid response error", func() {
		It("returns an error", func() {
			cfClient.GetLastOperationForInstanceReturns(cf.LastOperation{}, cf.NewInvalidResponseError("not valid json"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not valid json.", serviceInstance1GUID)))
		})
	})

	Context("after deleting", func() {
		Context("when get instances of service offering returns any instances", func() {
			It("returns an instances found error", func() {
				GetServiceInstancesCallCount := 0
				cfClient.GetServiceInstancesStub = func(filter cf.GetInstancesFilter, logger *log.Logger) ([]cf.Instance, error) {
					if GetServiceInstancesCallCount == 0 {
						GetServiceInstancesCallCount++
						return []cf.Instance{
							{GUID: serviceInstance1GUID},
							{GUID: serviceInstance2GUID},
						}, nil
					} else {
						return []cf.Instance{{GUID: "guid-that-shouldnt-be-there"}}, nil
					}
				}

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("expected 0 instances for service offering with unique ID: some-unique-service-id. Got 1 instance(s)."))
			})
		})

		Context("when get instances of service offerings returns an error", func() {
			It("returns the error", func() {
				GetServiceInstancesCallCount := 0
				cfClient.GetServiceInstancesStub = func(filter cf.GetInstancesFilter, logger *log.Logger) ([]cf.Instance, error) {
					if GetServiceInstancesCallCount == 0 {
						GetServiceInstancesCallCount++
						return []cf.Instance{
							{GUID: serviceInstance1GUID},
							{GUID: serviceInstance2GUID},
						}, nil
					} else {
						return []cf.Instance{}, errors.New("error getting instances")
					}
				}

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("error getting instances"))
			})
		})
	})
})
