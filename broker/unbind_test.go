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
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var _ = Describe("Unbind", func() {
	var (
		instanceID     string
		bindingID      string
		serviceID      string
		planID         string
		deploymentName string
		boshVms        bosh.BoshVMs
		actualManifest []byte
		secretsMap     map[string]string
		dnsDetails     map[string]string
		asyncAllowed   bool
	)

	BeforeEach(func() {
		instanceID = "a-most-unimpressive-instance"
		bindingID = "I'm still a binding"
		serviceID = "awesome-service"
		deploymentName = broker.InstancePrefix + instanceID
		boshVms = bosh.BoshVMs{"redis-server": []string{"an.ip"}}
		actualManifest = []byte("name: foo\npassword: ((/secret/path))")
		secretsMap = map[string]string{"/secret/path": "a73ghjdysj3"}
		dnsDetails = map[string]string{"config-1": "some.names.bosh"}
		asyncAllowed = false
		planID = existingPlanID

		boshClient.VMsReturns(boshVms, nil)
		serviceAdapter.DeleteBindingReturns(nil)
		fakeSecretManager.ResolveManifestSecretsReturns(secretsMap, nil)
		boshClient.GetDeploymentReturns(actualManifest, true, nil)
		boshClient.GetDNSAddressesReturns(dnsDetails, nil)

		b = createDefaultBroker()
	})

	It("succeeds with a synchronous request", func() {
		unbindResponse, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)
		Expect(unbindErr).NotTo(HaveOccurred())

		Expect(boshClient.VMsCallCount()).To(Equal(1))
		actualDeploymentName, _ := boshClient.VMsArgsForCall(0)
		Expect(actualDeploymentName).To(Equal(deploymentName))

		Expect(serviceAdapter.DeleteBindingCallCount()).To(Equal(1))
		passedBindingID, passedVms, passedManifest, passedRequestParams, passedSecretsMap, dnsAddresses, _ := serviceAdapter.DeleteBindingArgsForCall(0)
		Expect(passedBindingID).To(Equal(bindingID))
		Expect(passedVms).To(Equal(boshVms))
		Expect(passedManifest).To(Equal(actualManifest))
		Expect(passedRequestParams).To(Equal(map[string]interface{}{"service_id": serviceID, "plan_id": planID}))
		Expect(passedSecretsMap).To(Equal(secretsMap))
		Expect(dnsAddresses).To(Equal(dnsDetails))

		Expect(logBuffer.String()).To(MatchRegexp(
			`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\]`))

		Expect(unbindResponse.IsAsync).To(BeFalse())
	})

	It("acts synchronously even when async responses are allowed", func() {
		asyncAllowed = true
		unbindResponse, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)
		Expect(unbindErr).NotTo(HaveOccurred())
		Expect(unbindResponse.IsAsync).To(BeFalse())
	})

	It("preserves the uuid when one is provided through the ctx", func() {
		requestID := uuid.New()
		contextWithReqID := brokercontext.WithReqID(context.Background(), requestID)
		unbindDetails := domain.UnbindDetails{
			ServiceID: serviceID,
			PlanID:    planID,
		}
		b.Unbind(contextWithReqID, instanceID, bindingID, unbindDetails, false)

		Expect(logBuffer.String()).To(ContainSubstring(requestID))
	})

	It("fails when bosh fails to get VMs", func() {
		boshClient.VMsReturns(nil, errors.New("oops"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(MatchError(SatisfyAll(
			ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
			MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
			ContainSubstring("service: a-cool-redis-service"),
			ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
			ContainSubstring("operation: unbind"),
		)))

		Expect(unbindErr).NotTo(MatchError(ContainSubstring("task-id:")))
		Expect(logBuffer.String()).To(ContainSubstring("oops"))
	})

	It("fails when bosh client cannot find a deployment", func() {
		boshClient.GetDeploymentReturns(nil, false, nil)
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(HaveOccurred())
		Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("instance %s, not found", instanceID)))
	})

	It("fails when there is an error while fetching a manifest from the bosh client", func() {
		boshClient.GetDeploymentReturns(nil, false, fmt.Errorf("problem fetching manifest"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(HaveOccurred())
		Expect(logBuffer.String()).To(ContainSubstring("problem fetching manifest"))
	})

	It("fails when it cannot find the instance", func() {
		boshClient.GetDeploymentReturns(nil, false, nil)
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(Equal(apiresponses.ErrInstanceDoesNotExist))
	})

	It("logs a message but still calls unbind when bosh client cannot return variables for deployment", func() {
		boshClient.VariablesReturns(nil, errors.New("oops"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)
		Expect(unbindErr).NotTo(HaveOccurred())

		Expect(logBuffer.String()).To(ContainSubstring("failed to retrieve deployment variables"))
		Expect(serviceAdapter.DeleteBindingCallCount()).To(Equal(1))
	})

	It("fails when the plan cannot be found", func() {
		planID = "not-so-awesome-plan"
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(MatchError(`plan "not-so-awesome-plan" not found`))
	})

	It("fails when the bosh client fails to GetDNSAddresses", func() {
		boshClient.GetDNSAddressesReturns(nil, errors.New("'fraid not"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(MatchError(ContainSubstring("There was a problem completing your request.")))
		Expect(logBuffer.String()).To(ContainSubstring("failed to get required DNS info"))
	})

	It("logs a message but unbinds anyway when the secretManager cannot resolve manifest secrets", func() {
		fakeSecretManager.ResolveManifestSecretsReturns(nil, errors.New("oops"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).NotTo(HaveOccurred())
		Expect(logBuffer.String()).To(ContainSubstring("failed to resolve manifest secrets"))
		Expect(serviceAdapter.DeleteBindingCallCount()).To(Equal(1))
	})

	It("returns an generic error when the service adapter fails to destroy the binding returning a go standard error", func() {
		serviceAdapter.DeleteBindingReturns(errors.New("executing unbinding failed"))
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(MatchError(SatisfyAll(
			ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
			MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
			ContainSubstring("service: a-cool-redis-service"),
			ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
			ContainSubstring("operation: unbind"),
		)))

		Expect(unbindErr).NotTo(MatchError(ContainSubstring("task-id:")))
		Expect(logBuffer.String()).To(ContainSubstring("executing unbinding failed"))
	})

	It("returns an specific error when the service adapter fails to destroy the binding returning a serviceadapter error", func() {
		var err = serviceadapter.NewUnknownFailureError("it failed, but all is not lost dear user")
		serviceAdapter.DeleteBindingReturns(err)
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(Equal(err))
	})

	It("returns an error when the service adapter cannot find the binding", func() {
		serviceAdapter.DeleteBindingReturns(serviceadapter.BindingNotFoundError{})
		_, unbindErr := b.Unbind(context.Background(), instanceID, bindingID, domain.UnbindDetails{ServiceID: serviceID, PlanID: planID}, asyncAllowed)

		Expect(unbindErr).To(Equal(apiresponses.ErrBindingDoesNotExist))
	})
})
