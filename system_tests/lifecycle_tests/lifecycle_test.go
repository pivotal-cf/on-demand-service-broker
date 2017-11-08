// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("On-demand service broker", func() {
	newServiceName := func() string {
		return fmt.Sprintf("instance-%s", uuid.New()[:7])
	}

	unbindService := func(testAppName, serviceName string) {
		Eventually(cf.Cf("unbind-service", testAppName, serviceName), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	}

	testCrud := func(testAppURL string) {
		cf_helpers.PutToTestApp(testAppURL, "foo", "bar")
		Expect(cf_helpers.GetFromTestApp(testAppURL, "foo")).To(Equal("bar"))
	}

	testFifo := func(testAppURL string) {
		queue := "a-test-queue"
		cf_helpers.PushToTestAppQueue(testAppURL, queue, "foo")
		cf_helpers.PushToTestAppQueue(testAppURL, queue, "bar")
		Expect(cf_helpers.PopFromTestAppQueue(testAppURL, queue)).To(Equal("foo"))
		Expect(cf_helpers.PopFromTestAppQueue(testAppURL, queue)).To(Equal("bar"))
	}

	updatePlan := func(serviceName, updatedPlanName string) {
		Eventually(cf.Cf("update-service", serviceName, "-p", updatedPlanName), cf_helpers.CfTimeout).Should(gexec.Exit(0))
		cf_helpers.AwaitServiceUpdate(serviceName)
	}

	updateServiceWithArbParams := func(serviceName string, arbitraryParams json.RawMessage) {
		Eventually(cf.Cf("update-service", serviceName, "-c", string(arbitraryParams)), cf_helpers.CfTimeout).Should(gexec.Exit(0))
		cf_helpers.AwaitServiceUpdate(serviceName)
	}

	cfCmdOutput := func(cfArgs ...string) string {
		cmd := cf.Cf(cfArgs...)
		Eventually(cmd, cf_helpers.CfTimeout).Should(gexec.Exit(0))
		return string(cmd.Buffer().Contents())
	}

	getOAuthToken := func() string {
		oauthTokenOutput := cfCmdOutput("oauth-token")
		oauthTokenRe := regexp.MustCompile(`(?m)^bearer .*$`)
		authToken := oauthTokenRe.FindString(oauthTokenOutput)
		Expect(authToken).ToNot(BeEmpty())
		return authToken
	}

	testODBMetrics := func(brokerDeploymentName, serviceOfferingName, planName string) {
		Expect(dopplerAddress).NotTo(BeEmpty())
		firehoseConsumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
		firehoseConsumer.SetDebugPrinter(GinkgoFirehosePrinter{})
		defer firehoseConsumer.Close()

		msgChan, errChan := firehoseConsumer.Firehose("SystemTests-"+uuid.New(), getOAuthToken())
		timeoutChan := time.After(1 * time.Minute)

		for {
			select {
			case msg := <-msgChan:
				fmt.Fprintf(GinkgoWriter, "firehose: received message %+v\n", msg)
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
				Fail("timed out after 1 minute")
				return
			}
		}
	}

	testServiceWithExampleApp := func(exampleAppType, testAppURL string) {
		switch exampleAppType {
		case "crud":
			testCrud(testAppURL)
		case "fifo":
			testFifo(testAppURL)
		default:
			Fail(fmt.Sprintf("invalid example app type %s. valid types are: crud, fifo", exampleAppType))
		}
	}

	testCredhubRef := func(appName, serviceOffering string) {
		By("ensuring credential in app env is credhub-ref")
		bindingCredentials, err := cf_helpers.AppBindingCreds(appName, serviceOffering)
		Expect(err).NotTo(HaveOccurred())
		credMap, ok := bindingCredentials.(map[string]interface{})
		Expect(ok).To(BeTrue())
		credhubRef, ok := credMap["credhub-ref"].(string)
		Expect(ok).To(BeTrue(), fmt.Sprintf("unable to find credhub-ref in credentials %+v", credMap))
		Expect(credhubRef).To(ContainSubstring("/c/%s", serviceOffering))
	}

	lifecycle := func(t LifecycleTest) {
		It("supports the lifecycle of a service instance", func() {
			By(fmt.Sprintf("allowing creation of a service instance with plan: '%s' and arbitrary params: '%s'", t.Plan, string(t.ArbitraryParams)))
			testAppName := uuid.New()[:7]
			serviceName := newServiceName()
			cf_helpers.CreateService(serviceOffering, t.Plan, serviceName, string(t.ArbitraryParams))

			By("allowing an app to bind to the service instance")
			testAppURL := cf_helpers.PushAndBindApp(testAppName, serviceName, exampleAppPath)
			defer func() {
				Eventually(cf.Cf(
					"delete",
					testAppName,
					"-f",
					"-r",
				), cf_helpers.CfTimeout).Should(gexec.Exit())
			}()

			if shouldTestCredhubRef {
				testCredhubRef(testAppName, serviceOffering)
			}

			By("creating a service key")
			serviceKeyName := uuid.New()[:7]
			cf_helpers.CreateServiceKey(serviceName, serviceKeyName)
			serviceKey := cf_helpers.GetServiceKey(serviceName, serviceKeyName)
			Expect(serviceKey).NotTo(BeNil())

			By("providing a functional service instance")
			testServiceWithExampleApp(exampleAppType, testAppURL)

			if shouldTestODBMetrics {
				By("emitting metrics to the CF firehose")
				testODBMetrics(brokerDeploymentName, serviceOffering, t.Plan)
			}

			if t.UpdateToPlan != "" {
				By(fmt.Sprintf("allowing to update the service instance to plan: '%s'", t.UpdateToPlan))
				updatePlan(serviceName, t.UpdateToPlan)

				By("providing a functional service instance post-update")
				testServiceWithExampleApp(exampleAppType, testAppURL)
			}

			if len(t.ArbitraryParams) > 0 {
				By(fmt.Sprintf("allowing to update the service instance with arbitrary params: '%s'", string(t.ArbitraryParams)))
				updateServiceWithArbParams(serviceName, t.ArbitraryParams)

				By("providing a functional service instance post-update")
				testServiceWithExampleApp(exampleAppType, testAppURL)
			}

			By("allowing the app to be unbound from the service instance")
			unbindService(testAppName, serviceName)

			By("deleting the service key")
			Eventually(
				cf.Cf("delete-service-key", "-f", serviceName, serviceKeyName),
				cf_helpers.CfTimeout,
			).Should(gexec.Exit(0))

			By("allowing the service instance to be deleted")
			cf_helpers.DeleteService(serviceName)
		})
	}

	for _, test := range tests {
		lifecycle(test)
	}
})

type GinkgoFirehosePrinter struct{}

func (c GinkgoFirehosePrinter) Print(title, dump string) {
	fmt.Fprintf(GinkgoWriter, "firehose: %s\n---%s\n---\n", title, dump)
}
