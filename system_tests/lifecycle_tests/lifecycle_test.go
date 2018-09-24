// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"
	"time"

	"encoding/json"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var credhubPath = "/odb/sys/test/secret-" + uuid.New()[:7]

var _ = Describe("On-demand service broker", func() {
	var (
		testAppName    string
		serviceName    string
		serviceKeyName string
	)

	BeforeEach(func() {
		testAppName = uuid.New()[:7]
		serviceName = newServiceName()
		serviceKeyName = uuid.New()[:7]

		if secureManifestsEnabled {
			credhubCLI.SetCredhubValueFor(credhubPath)
		}
	})

	AfterEach(func() {
		Eventually(cf.Cf("unbind-service", testAppName, serviceName), cf.CfTimeout).Should(gexec.Exit())
		Eventually(cf.Cf("delete", testAppName, "-f", "-r"), cf.CfTimeout).Should(gexec.Exit())

		Eventually(
			cf.Cf("delete-service-key", "-f", serviceName, serviceKeyName),
			cf.CfTimeout,
		).Should(gexec.Exit())

		cf.DeleteService(serviceName)

		if secureManifestsEnabled {
			credhubCLI.DeleteCredhubValueFor(credhubPath)
		}
	})

	lifecycle := func(t LifecycleTest) {
		It("supports the lifecycle of a service instance", func() {
			rawArbitraryParams := prepareArbitraryParams(t)

			By(fmt.Sprintf("allowing creation of a service instance with plan: '%s' and arbitrary params: '%s'", t.Plan, rawArbitraryParams))
			cf.CreateService(serviceOffering, t.Plan, serviceName, rawArbitraryParams)

			By("creating a service key")
			cf.CreateServiceKey(serviceName, serviceKeyName)
			serviceKey := cf.GetServiceKey(serviceName, serviceKeyName)
			Expect(serviceKey).NotTo(BeNil())

			if t.BindingDNSAttribute != "" && shouldTestBindingWithDNS {
				testBindingWithDNS(serviceKey, t.BindingDNSAttribute)
			}

			By("allowing an app to bind to the service instance")
			testAppURL := cf.PushAndBindApp(testAppName, serviceName, exampleAppPath)

			if shouldTestCredhubRef {
				By("ensuring credential in app env is credhub-ref", func() {
					bindingCredentials, err := cf.AppBindingCreds(testAppName, serviceOffering)
					Expect(err).NotTo(HaveOccurred())
					credMap, ok := bindingCredentials.(map[string]interface{})
					Expect(ok).To(BeTrue())
					credhubRef, ok := credMap["credhub-ref"].(string)
					Expect(ok).To(BeTrue(), fmt.Sprintf("unable to find credhub-ref in credentials %+v", credMap))
					Expect(credhubRef).To(ContainSubstring("/c/%s", serviceID))
				})
			}

			By("providing a functional service instance")
			testServiceWithExampleApp(exampleAppType, testAppURL)

			if shouldTestODBMetrics {
				By("emitting metrics to the CF firehose")
				testODBMetrics(brokerDeploymentName, serviceOffering, t.Plan)
			}

			var odbSecret map[string]string
			if secureManifestsEnabled {
				serviceInstanceGUID := cf.GetServiceInstanceGUID(serviceName)
				odbSecret = credhubCLI.GetCredhubValueFor(serviceOffering, serviceInstanceGUID, "odb_managed_secret")
				Expect(odbSecret["value"]).To(Equal("HardcodedAdapterValue"))
			}

			if t.UpdateToPlan != "" {
				By(fmt.Sprintf("allowing to update the service instance to plan: '%s'", t.UpdateToPlan))

				Eventually(
					cf.Cf("update-service", serviceName, "-p", t.UpdateToPlan, "-c", `{"odb_managed_secret": "newValue"}`),
					cf.CfTimeout,
				).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(serviceName)

				By("providing a functional service instance post-update")
				testServiceWithExampleApp(exampleAppType, testAppURL)

				if secureManifestsEnabled {
					By("updating the value of the credhub stored secret")
					serviceInstanceGUID := cf.GetServiceInstanceGUID(serviceName)
					newOdbSecret := credhubCLI.GetCredhubValueFor(serviceOffering, serviceInstanceGUID, "odb_managed_secret")
					Expect(newOdbSecret["name"]).To(Equal(odbSecret["name"]))
					Expect(newOdbSecret["value"]).To(Equal("newValue"))
				}
			}

			if len(t.ArbitraryParams) > 0 {
				By(fmt.Sprintf("allowing to update the service instance with arbitrary params: '%s'", rawArbitraryParams))
				Eventually(cf.Cf("update-service", serviceName, "-c", rawArbitraryParams), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(serviceName)

				By("providing a functional service instance post-update")
				testServiceWithExampleApp(exampleAppType, testAppURL)
			}

			By("allowing the app to be unbound from the service instance")
			Eventually(cf.Cf("unbind-service", testAppName, serviceName), cf.CfTimeout).Should(gexec.Exit(0))

			By("deleting the service key")
			Eventually(
				cf.Cf("delete-service-key", "-f", serviceName, serviceKeyName),
				cf.CfTimeout,
			).Should(gexec.Exit(0))

			By("allowing the service instance to be deleted")
			cf.DeleteService(serviceName)
		})
	}

	for _, test := range tests {
		lifecycle(test)
	}
})

func toJsonString(obj interface{}) string {
	b, err := json.Marshal(obj)
	Expect(err).ToNot(HaveOccurred())
	return string(b)
}

func addCredhubPathToArbitraryParams(arbitraryParams map[string]interface{}, credhubPath string) map[string]interface{} {
	arbitraryParams["credhub_secret_path"] = credhubPath
	return arbitraryParams
}

func prepareArbitraryParams(t LifecycleTest) string {
	arbitraryParams := t.ArbitraryParams
	if t.HasCredhubSecretPath {
		arbitraryParams = addCredhubPathToArbitraryParams(arbitraryParams, credhubPath)
	}
	return toJsonString(arbitraryParams)
}

func newServiceName() string {
	return fmt.Sprintf("instance-%s", uuid.New()[:7])
}

func testCrud(testAppURL string) {
	cf.PutToTestApp(testAppURL, "foo", "bar")
	Expect(cf.GetFromTestApp(testAppURL, "foo")).To(Equal("bar"))
}

func testFifo(testAppURL string) {
	queue := "a-test-queue"
	cf.PushToTestAppQueue(testAppURL, queue, "foo")
	cf.PushToTestAppQueue(testAppURL, queue, "bar")
	Expect(cf.PopFromTestAppQueue(testAppURL, queue)).To(Equal("foo"))
	Expect(cf.PopFromTestAppQueue(testAppURL, queue)).To(Equal("bar"))
}

func getOAuthToken() string {
	cmd := cf.Cf("oauth-token")
	Eventually(cmd, cf.CfTimeout).Should(gexec.Exit(0))
	oauthTokenOutput := string(cmd.Buffer().Contents())
	oauthTokenRe := regexp.MustCompile(`(?m)^bearer .*$`)
	authToken := oauthTokenRe.FindString(oauthTokenOutput)
	Expect(authToken).ToNot(BeEmpty())
	return authToken
}

func testODBMetrics(brokerDeploymentName, serviceOfferingName, planName string) {
	Expect(dopplerAddress).NotTo(BeEmpty())
	firehoseConsumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
	firehoseConsumer.SetDebugPrinter(GinkgoFirehosePrinter{})
	defer firehoseConsumer.Close()

	msgChan, errChan := firehoseConsumer.Firehose("SystemTests-"+uuid.New(), getOAuthToken())
	timeoutChan := time.After(5 * time.Minute)

	for {
		select {
		case msg := <-msgChan:
			// fmt.Fprintf(GinkgoWriter, "firehose: received message %+v\n", msg)
			if msg != nil && *msg.EventType == events.Envelope_ValueMetric && strings.HasSuffix(*msg.Deployment, brokerDeploymentName) {
				fmt.Fprintf(GinkgoWriter, "received metric for deployment %s: %+v\n", brokerDeploymentName, msg)
				if msg.ValueMetric.GetName() == fmt.Sprintf("/on-demand-broker/%s/%s/total_instances", serviceOfferingName, planName) {
					fmt.Fprintln(GinkgoWriter, "ODB metrics test successful")
					return
				}
			}
		case err := <-errChan:
			Expect(err).NotTo(HaveOccurred())
			return
		case <-timeoutChan:
			Fail("timed out after 5 minute")
			return
		}
	}
}

func testServiceWithExampleApp(exampleAppType, testAppURL string) {
	switch exampleAppType {
	case "crud":
		testCrud(testAppURL)
	case "fifo":
		testFifo(testAppURL)
	default:
		Fail(fmt.Sprintf("invalid example app type %s. valid types are: crud, fifo", exampleAppType))
	}
}

func testBindingWithDNS(serviceKeyRaw, bindingDNSAttribute string) {
	serviceKeyWithoutMessageSlice := strings.Split(serviceKeyRaw, "\n")[1:]
	onlyServiceKey := strings.Join(serviceKeyWithoutMessageSlice, "\n")
	var serviceKey map[string]interface{}
	json.Unmarshal([]byte(onlyServiceKey), &serviceKey)

	dnsInfo, ok := serviceKey[bindingDNSAttribute]
	Expect(ok).To(BeTrue(), fmt.Sprintf("%s not returned in binding", bindingDNSAttribute))

	dnsInfoMap, ok := dnsInfo.(map[string]interface{})
	Expect(ok).To(BeTrue(), fmt.Sprintf("Unable to convert dns info to map[string]interface{}, got:%t", dnsInfo))

	Expect(len(dnsInfoMap)).To(BeNumerically(">", 0))
}

type GinkgoFirehosePrinter struct{}

func (c GinkgoFirehosePrinter) Print(title, dump string) {
	fmt.Fprintf(GinkgoWriter, "firehose: %s\n---%s\n---\n", title, dump)
}
