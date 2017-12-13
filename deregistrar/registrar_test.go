package deregistrar_test

import (
	"fmt"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/deregistrar"
	"github.com/pivotal-cf/on-demand-service-broker/deregistrar/fakes"

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

		var deregistrarConfig deregistrar.Config
		err = yaml.Unmarshal(configFileBytes, &deregistrarConfig)
		Expect(err).NotTo(HaveOccurred())

		expected := deregistrar.Config{
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL:         "some-cf-url",
				TrustedCert: "some-cf-cert",
				Authentication: config.Authentication{
					UAA: config.UAAAuthentication{
						URL: "a-uaa-url",
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

	It("does not return an error when deregistering", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)

		registrar := deregistrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).NotTo(HaveOccurred())
		Expect(fakeCFClient.GetServiceOfferingGUIDCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerArgsForCall(0)).To(Equal(brokerGUID))
	})

	It("returns an error when cf client fails to ge the service offering guid", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns("", errors.New("list service broker failed"))

		registrar := deregistrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).To(MatchError("list service broker failed"))
	})

	It("returns an error when cf client fails to deregister", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)
		fakeCFClient.DeregisterBrokerReturns(errors.New("failed"))

		registrar := deregistrar.New(fakeCFClient, nil)

		errMsg := fmt.Sprintf("Failed to deregister broker with %s with guid %s, err: failed", brokerName, brokerGUID)
		Expect(registrar.Deregister(brokerName)).To(MatchError(errMsg))
	})
})
