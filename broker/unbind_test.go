// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var _ = Describe("Unbind", func() {
	var (
		instanceID     = "a-most-unimpressive-instance"
		bindingID      = "I'm still a binding"
		serviceID      = "awesome-service"
		planID         = "awesome-plan"
		deploymentName = broker.InstancePrefix + instanceID
		boshVms        = bosh.BoshVMs{"redis-server": []string{"an.ip"}}
		actualManifest = []byte("a valid manifest")
		unbindErr      error
	)

	BeforeEach(func() {
		boshClient.VMsReturns(boshVms, nil)
		serviceAdapter.DeleteBindingReturns(nil)
		boshClient.GetDeploymentReturns(actualManifest, true, nil)
	})

	JustBeforeEach(func() {
		unbindErr = b.Unbind(context.Background(), instanceID, bindingID, brokerapi.UnbindDetails{ServiceID: serviceID, PlanID: planID})
	})

	It("asks bosh for VMs from a deployment named by the manifest generator", func() {
		Expect(boshClient.VMsCallCount()).To(Equal(1))
		actualDeploymentName, _ := boshClient.VMsArgsForCall(0)
		Expect(actualDeploymentName).To(Equal(deploymentName))
	})

	It("destroys the binding using the bosh topology and admin credentials", func() {
		Expect(serviceAdapter.DeleteBindingCallCount()).To(Equal(1))
		passedBindingID, passedVms, passedManifest, passedRequestParams, _ := serviceAdapter.DeleteBindingArgsForCall(0)
		Expect(passedBindingID).To(Equal(bindingID))
		Expect(passedVms).To(Equal(boshVms))
		Expect(passedManifest).To(Equal(actualManifest))
		Expect(passedRequestParams).To(Equal(map[string]interface{}{"service_id": serviceID, "plan_id": planID}))
	})

	It("does not error", func() {
		Expect(unbindErr).NotTo(HaveOccurred())
	})

	It("logs using a request ID", func() {
		Expect(logBuffer.String()).To(MatchRegexp(fmt.Sprintf(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} service adapter will delete binding with ID %s for instance %s`, bindingID, instanceID)))
	})

	Context("when bosh fails to get VMs", func() {
		BeforeEach(func() {
			boshClient.VMsReturns(nil, errors.New("oops"))
		})

		Describe("returned error", func() {
			It("includes a standard message", func() {
				Expect(unbindErr).To(MatchError(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information:",
				)))
			})

			It("includes the broker request id", func() {
				Expect(unbindErr).To(MatchError(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				)))
			})

			It("includes the service name", func() {
				Expect(unbindErr).To(MatchError(ContainSubstring(
					"service: a-cool-redis-service",
				)))
			})

			It("includes a service instance guid", func() {
				Expect(unbindErr).To(MatchError(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				)))
			})

			It("includes the operation type", func() {
				Expect(unbindErr).To(MatchError(ContainSubstring(
					"operation: unbind",
				)))
			})

			It("does NOT include the bosh task id", func() {
				Expect(unbindErr).NotTo(MatchError(ContainSubstring(
					"task-id:",
				)))
			})
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("oops"))
		})
	})

	Context("when bosh client cannot find a deployment", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns(nil, false, nil)
		})

		It("returns an error", func() {
			Expect(unbindErr).To(HaveOccurred())
		})
	})

	Context("when bosh client returns a request error", func() {
		BeforeEach(func() {
			boshClient.VMsReturns(nil, boshdirector.NewRequestError(errors.New("bosh down.")))
		})

		It("logs the error", func() {
			Expect(logBuffer.String()).To(ContainSubstring("error: could not get deployment info: bosh down."))
		})

		It("returns the try again later error for the user", func() {
			Expect(unbindErr).To(MatchError(ContainSubstring("Currently unable to unbind service instance, please try again later")))
		})
	})

	Context("when there is an error while fetching a manifest from the bosh client", func() {
		BeforeEach(func() {
			boshClient.GetDeploymentReturns(nil, false, fmt.Errorf("problem fetching manifest"))
		})

		It("returns an error", func() {
			Expect(unbindErr).To(HaveOccurred())
		})
	})

	Context("when cannot find the instance", func() {
		BeforeEach(func() {
			boshClient.VMsReturns(bosh.BoshVMs{}, boshdirector.DeploymentNotFoundError{})
		})

		It("returns an error", func() {
			Expect(unbindErr).To(Equal(brokerapi.ErrInstanceDoesNotExist))
		})
	})

	Context("when the service adapter fails to destroy the binding", func() {
		Context("with no message for the user", func() {
			BeforeEach(func() {
				serviceAdapter.DeleteBindingReturns(errors.New("executing unbinding failed"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(unbindErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(unbindErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(unbindErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes a service instance guid", func() {
					Expect(unbindErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(unbindErr).To(MatchError(ContainSubstring(
						"operation: unbind",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(unbindErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error for the operator", func() {
				Expect(logBuffer.String()).To(ContainSubstring("executing unbinding failed"))
			})
		})

		Context("with an error message for the user", func() {
			var err = serviceadapter.NewUnknownFailureError("it failed, but all is not lost dear user")

			BeforeEach(func() {
				serviceAdapter.DeleteBindingReturns(err)
			})

			It("returns the user error", func() {
				Expect(unbindErr).To(Equal(err))
			})
		})
	})

	Context("when the service adapter cannot find the binding", func() {
		BeforeEach(func() {
			serviceAdapter.DeleteBindingReturns(serviceadapter.BindingNotFoundError{})
		})

		It("returns a binding not found error", func() {
			Expect(unbindErr).To(Equal(brokerapi.ErrBindingDoesNotExist))
		})
	})
})
