// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registrar_test

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"github.com/pivotal-cf/on-demand-service-broker/registrar/fakes"

	"errors"

	"os"

	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deregistrar Config", func() {
	It("loads valid config", func() {
		cwd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		configFilePath := filepath.Join(cwd, "fixtures", "deregister_config.yml")

		configFileBytes, err := ioutil.ReadFile(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		var deregistrarConfig registrar.Config
		err = yaml.Unmarshal(configFileBytes, &deregistrarConfig)
		Expect(err).NotTo(HaveOccurred())

		expected := registrar.Config{
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL:         "some-cf-url",
				TrustedCert: "some-cf-cert",
				UAA: config.UAAConfig{
					URL: "a-uaa-url",
					Authentication: config.UAACredentials{
						UserCredentials: config.UserCredentials{
							Username: "some-cf-username",
							Password: "some-cf-password",
						},
					},
				},
			},
		}

		Expect(expected).To(Equal(deregistrarConfig))
	})
})

var _ = Describe("Deregistrar", func() {
	const (
		brokerGUID = "broker-guid"
		brokerName = "broker-name"
	)

	var fakeCFClient *fakes.FakeCloudFoundryClient

	BeforeEach(func() {
		fakeCFClient = new(fakes.FakeCloudFoundryClient)
	})

	It("succeeds when deregistering succeeds", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)
		registrar := registrar.New(fakeCFClient, nil)

		err := registrar.Deregister(brokerName)

		Expect(err).NotTo(HaveOccurred())
		Expect(fakeCFClient.GetServiceOfferingGUIDCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerArgsForCall(0)).To(Equal(brokerGUID))
	})

	It("succeeds when the broker does not exist", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns("", nil)
		registrar := registrar.New(fakeCFClient, nil)

		err := registrar.Deregister(brokerName)

		Expect(err).NotTo(HaveOccurred())
		Expect(fakeCFClient.GetServiceOfferingGUIDCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerCallCount()).To(BeZero())
	})

	It("returns an error when cf client fails to get the service offering guid", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns("", errors.New("list service broker failed"))

		registrar := registrar.New(fakeCFClient, nil)
		err := registrar.Deregister(brokerName)

		Expect(err).To(MatchError("list service broker failed"))
	})

	It("returns an error when cf client fails to deregister", func() {
		errMsg := fmt.Sprintf("Failed to deregister broker with %s with guid %s, err: failed", brokerName, brokerGUID)
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)
		fakeCFClient.DeregisterBrokerReturns(errors.New("failed"))

		registrar := registrar.New(fakeCFClient, nil)
		err := registrar.Deregister(brokerName)

		Expect(err).To(MatchError(errMsg))
	})
})
