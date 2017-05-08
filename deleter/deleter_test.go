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
	"github.com/pivotal-cf/on-demand-service-broker/cloud_foundry_client"
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

		serviceKey cloud_foundry_client.ServiceKey
		binding    cloud_foundry_client.Binding
	)

	BeforeEach(func() {
		logBuffer = new(bytes.Buffer)
		logger = loggerfactory.
			New(io.MultiWriter(GinkgoWriter, logBuffer), "[deleter-unit-tests] ", log.LstdFlags).
			NewWithRequestID()

		cfClient = new(fakes.FakeCloudFoundryClient)

		cfClient.GetInstancesOfServiceOfferingReturns([]string{serviceInstance1GUID}, nil)

		binding = cloud_foundry_client.Binding{
			GUID:    serviceInstance1BindingGUID,
			AppGUID: serviceInstance1BoundAppGUID,
		}
		cfClient.GetBindingsForInstanceReturns([]cloud_foundry_client.Binding{binding}, nil)

		serviceKey = cloud_foundry_client.ServiceKey{
			GUID: serviceInstance1KeyGUID,
		}
		cfClient.GetServiceKeysForInstanceReturns([]cloud_foundry_client.ServiceKey{serviceKey}, nil)

		notFoundError := cloud_foundry_client.NewResourceNotFoundError("service instance not found")
		cfClient.GetInstanceReturns(cloud_foundry_client.Instance{}, notFoundError)

		sleeper = new(fakes.FakeSleeper)
		deleteTool = deleter.New(cfClient, sleeper, pollingInitialOffset, pollingInterval, logger)
	})

	It("logs its polling configuration at startup", func() {
		deleteTool.DeleteAllServiceInstances(serviceUniqueID)
		Expect(logBuffer.String()).To(ContainSubstring("Deleter Configuration: polling_intial_offset: %d, polling_interval: %d.", pollingInitialOffset, pollingInterval))
	})

	Context("when no service instances exist", func() {
		It("logs that there are no instances", func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{}, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).NotTo(HaveOccurred())

			Expect(logBuffer.String()).To(ContainSubstring("No service instances found."))

			Expect(cfClient.GetInstancesOfServiceOfferingCallCount()).To(Equal(1), "Expected to call get instances only once")
		})
	})

	Context("when it succeeds", func() {
		Context("when two service instances are deleted immediately", func() {
			BeforeEach(func() {
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{
					serviceInstance1GUID,
					serviceInstance2GUID,
				}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)
				cfClient.GetBindingsForInstanceReturnsOnCall(0, []cloud_foundry_client.Binding{binding}, nil)
				cfClient.GetBindingsForInstanceReturnsOnCall(1, []cloud_foundry_client.Binding{}, nil)
				cfClient.GetServiceKeysForInstanceReturnsOnCall(0, []cloud_foundry_client.ServiceKey{serviceKey}, nil)
				cfClient.GetServiceKeysForInstanceReturnsOnCall(1, []cloud_foundry_client.ServiceKey{}, nil)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("by deleting instances", func() {
				By("By getting all instances for the service offering")

				actualServiceUniqueID, actualLogger := cfClient.GetInstancesOfServiceOfferingArgsForCall(0)
				Expect(actualServiceUniqueID).To(Equal(serviceUniqueID))
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
				actualInstanceGUID, _ = cfClient.GetInstanceArgsForCall(0)
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
				actualInstanceGUID, _ = cfClient.GetInstanceArgsForCall(1)
				Expect(actualInstanceGUID).To(Equal(serviceInstance2GUID))

				By("Verifying that all service instances have been deleted")

				actualServiceUniqueID, actualLogger = cfClient.GetInstancesOfServiceOfferingArgsForCall(1)
				Expect(actualServiceUniqueID).To(Equal(serviceUniqueID))
				Expect(actualLogger).To(Equal(logger))
			})
		})

		Context("when the service instance is not deleted immediately", func() {
			BeforeEach(func() {
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{serviceInstance1GUID}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)

				instance := cloud_foundry_client.Instance{
					LastOperation: cloud_foundry_client.LastOperation{
						Type:  cloud_foundry_client.OperationTypeDelete,
						State: cloud_foundry_client.OperationState("in progress"),
					},
				}
				cfClient.GetInstanceReturnsOnCall(0, instance, nil)
				notFoundError := cloud_foundry_client.NewResourceNotFoundError("service instance not found")
				cfClient.GetInstanceReturnsOnCall(1, cloud_foundry_client.Instance{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the instance", func() {
				Expect(cfClient.GetInstanceCallCount()).To(Equal(2), "Expected to get instance two times")

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
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{serviceInstance1GUID}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)

				instance := cloud_foundry_client.Instance{
					LastOperation: cloud_foundry_client.LastOperation{
						Type:  cloud_foundry_client.OperationTypeDelete,
						State: cloud_foundry_client.OperationState("succeeded"),
					},
				}
				cfClient.GetInstanceReturnsOnCall(0, instance, nil)
				notFoundError := cloud_foundry_client.NewResourceNotFoundError("service instance not found")
				cfClient.GetInstanceReturnsOnCall(1, cloud_foundry_client.Instance{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("by deleting instances", func() {
				Expect(cfClient.GetInstanceCallCount()).To(Equal(2), "Expected to get instance two times")

				Expect(logBuffer.String()).To(ContainSubstring("Result: deleted service instance %s", serviceInstance1GUID))
			})
		})

		Context("when get bindings returns a resource not found error", func() {
			It("skips deleting the binding and continues", func() {
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{serviceInstance1GUID}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)

				cfClient.GetBindingsForInstanceReturns(
					[]cloud_foundry_client.Binding{},
					cloud_foundry_client.NewResourceNotFoundError("no instance!"),
				)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfClient.DeleteBindingCallCount()).To(Equal(0))
			})
		})

		Context("when get service keys returns a resource not found error", func() {
			It("skips deleting the binding and continues", func() {
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{serviceInstance1GUID}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)

				cfClient.GetServiceKeysForInstanceReturns(
					[]cloud_foundry_client.ServiceKey{},
					cloud_foundry_client.NewResourceNotFoundError("no instance!"),
				)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfClient.DeleteServiceKeyCallCount()).To(Equal(0))
			})
		})

		Context("when the get service instance request fails", func() {
			BeforeEach(func() {
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(0, []string{serviceInstance1GUID}, nil)
				cfClient.GetInstancesOfServiceOfferingReturnsOnCall(1, []string{}, nil)

				cfClient.GetInstanceReturnsOnCall(0, cloud_foundry_client.Instance{}, errors.New("request failed"))
				notFoundError := cloud_foundry_client.NewResourceNotFoundError("service instance not found")
				cfClient.GetInstanceReturnsOnCall(1, cloud_foundry_client.Instance{}, notFoundError)

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("continues polling", func() {
				Expect(cfClient.GetInstanceCallCount()).To(Equal(2), "Expected to get instance two times")

				Expect(logBuffer.String()).To(ContainSubstring("Result: deleted service instance %s", serviceInstance1GUID))
			})
		})
	})

	Context("when get all service instances returns an error", func() {
		It("returns an error", func() {
			cfClient.GetInstancesOfServiceOfferingReturns([]string{}, errors.New("cannot get instances"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot get instances"))
		})
	})

	Context("when get bindings returns a non-ResourceNotFound error", func() {
		It("returns an error", func() {
			cfClient.GetBindingsForInstanceReturns([]cloud_foundry_client.Binding{}, errors.New("error getting bindings"))

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
			cfClient.GetServiceKeysForInstanceReturns([]cloud_foundry_client.ServiceKey{}, errors.New("error getting service keys"))

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
			instance := cloud_foundry_client.Instance{
				LastOperation: cloud_foundry_client.LastOperation{
					Type:  cloud_foundry_client.OperationTypeDelete,
					State: cloud_foundry_client.OperationStateFailed,
				},
			}
			cfClient.GetInstanceReturns(instance, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Delete operation failed.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns the wrong last operation type", func() {
		It("returns an error", func() {
			instance := cloud_foundry_client.Instance{
				LastOperation: cloud_foundry_client.LastOperation{
					Type:  cloud_foundry_client.OperationType("update"),
					State: cloud_foundry_client.OperationStateFailed,
				},
			}
			cfClient.GetInstanceReturns(instance, nil)

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Unexpected operation type: 'update'.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns an unauthorized error", func() {
		It("returns an error", func() {
			cfClient.GetInstanceReturns(cloud_foundry_client.Instance{}, cloud_foundry_client.NewUnauthorizedError("not logged in"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not logged in.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns a forbidden error", func() {
		It("returns an error", func() {
			cfClient.GetInstanceReturns(cloud_foundry_client.Instance{}, cloud_foundry_client.NewForbiddenError("not permitted"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not permitted.", serviceInstance1GUID)))
		})
	})

	Context("when get service instance returns an invalid response error", func() {
		It("returns an error", func() {
			cfClient.GetInstanceReturns(cloud_foundry_client.Instance{}, cloud_foundry_client.NewInvalidResponseError("not valid json"))

			err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Result: failed to delete service instance %s. Error: not valid json.", serviceInstance1GUID)))
		})
	})

	Context("after deleting", func() {
		Context("when get instances of service offering returns any instances", func() {
			It("returns an instances found error", func() {
				getInstancesCallCount := 0
				cfClient.GetInstancesOfServiceOfferingStub = func(serviceOfferingID string, logger *log.Logger) ([]string, error) {
					if getInstancesCallCount == 0 {
						getInstancesCallCount++
						return []string{
							serviceInstance1GUID,
							serviceInstance2GUID,
						}, nil
					} else {
						return []string{"guid-that-shouldnt-be-there"}, nil
					}
				}

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("expected 0 instances for service offering with unique ID: some-unique-service-id. Got 1 instance(s)."))
			})
		})

		Context("when get instances of service offerings returns an error", func() {
			It("returns the error", func() {
				getInstancesCallCount := 0
				cfClient.GetInstancesOfServiceOfferingStub = func(serviceOfferingID string, logger *log.Logger) ([]string, error) {
					if getInstancesCallCount == 0 {
						getInstancesCallCount++
						return []string{
							serviceInstance1GUID,
							serviceInstance2GUID,
						}, nil
					} else {
						return []string{}, errors.New("error getting instances")
					}
				}

				err := deleteTool.DeleteAllServiceInstances(serviceUniqueID)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("error getting instances"))
			})
		})
	})
})
