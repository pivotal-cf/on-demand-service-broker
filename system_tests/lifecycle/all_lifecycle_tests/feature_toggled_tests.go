package all_lifecycle_tests

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func FeatureToggledLifecycleTest(
	serviceType service_helpers.ServiceType,
	brokerInfo bosh_helpers.BrokerInfo,
	plan string,
	newPlanName string,
	arbitraryParams string,
	dopplerAddress string) {

	var (
		serviceInstanceName string
		serviceKeyContents  string
		serviceKeyName      string
		appName             string
		appURL              string
	)

	By("creating a service", func() {
		serviceInstanceName = "service" + brokerInfo.TestSuffix
		cf_helpers.CreateService(brokerInfo.ServiceOffering, plan, serviceInstanceName, "")
	})

	By("creating a service key", func() {
		serviceKeyName = "serviceKey" + brokerInfo.TestSuffix
		cf_helpers.CreateServiceKey(serviceInstanceName, serviceKeyName)
		serviceKeyContents = cf_helpers.GetServiceKey(serviceInstanceName, serviceKeyName)

		looksLikeAServiceKey(serviceKeyContents)
	})

	By("testing binding with DNS", func() {
		testBindingWithDNS(serviceKeyContents, "dns_addresses")
	})

	By("binding an app", func() {
		appName = "example-app" + brokerInfo.TestSuffix
		appPath := cf_helpers.GetAppPath(serviceType)
		appURL = cf_helpers.PushAndBindApp(appName, serviceInstanceName, appPath)
	})

	By("ensuring the binding is a runtime credhub reference", func() {
		testSecureBindings(brokerInfo, appName)
	})

	By("testing the broker emits metrics", func() {
		testMetrics(brokerInfo, plan, dopplerAddress)
	})

	By("testing the app can communicate with service", func() {
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating the plan for the service", func() {
		cf_helpers.UpdateServiceToPlan(serviceInstanceName, newPlanName)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating arbitrary parameters for the service", func() {
		cf_helpers.UpdateServiceWithArbitraryParams(serviceInstanceName, arbitraryParams)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("unbinding the app", func() {
		cf_helpers.UnbindAndDeleteApp(appName, serviceInstanceName)
	})

	By("deleting a service key", func() {
		cf_helpers.DeleteServiceKey(serviceInstanceName, serviceKeyName)
	})

	By("deleting the service", func() {
		cf_helpers.DeleteService(serviceInstanceName)
	})
}

func testBindingWithDNS(serviceKeyRaw, bindingDNSAttribute string) {
	var serviceKey map[string]interface{}
	err := json.Unmarshal([]byte(serviceKeyRaw), &serviceKey)
	Expect(err).ToNot(HaveOccurred())

	dnsInfo, ok := serviceKey[bindingDNSAttribute]
	Expect(ok).To(BeTrue(), fmt.Sprintf("%s not returned in binding", bindingDNSAttribute))

	dnsInfoMap, ok := dnsInfo.(map[string]interface{})
	Expect(ok).To(BeTrue(), fmt.Sprintf("Unable to convert dns info to map[string]interface{}, got:%t", dnsInfo))

	Expect(len(dnsInfoMap)).To(BeNumerically(">", 0))
}

func testSecureBindings(brokerInfo bosh_helpers.BrokerInfo, appName string) {
	bindingCredentials, err := cf_helpers.AppBindingCreds(appName, brokerInfo.ServiceOffering)
	Expect(err).NotTo(HaveOccurred())
	credMap, ok := bindingCredentials.(map[string]interface{})
	Expect(ok).To(BeTrue())
	credhubRef, ok := credMap["credhub-ref"].(string)
	Expect(ok).To(BeTrue(), fmt.Sprintf("unable to find credhub-ref in credentials %+v", credMap))
	Expect(credhubRef).To(ContainSubstring("/c/%s", brokerInfo.ServiceOffering))
}

func testMetrics(brokerInfo bosh_helpers.BrokerInfo, plan string, dopplerAddress string) {
	planName := plan
	brokerDeploymentName := brokerInfo.DeploymentName
	serviceOfferingName := brokerInfo.ServiceOffering
	Expect(dopplerAddress).NotTo(BeEmpty())

	firehoseConsumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
	defer firehoseConsumer.Close()

	msgChan, errChan := firehoseConsumer.Firehose("SystemTests-"+uuid.New(), cf_helpers.GetOAuthToken())
	timeoutChan := time.After(5 * time.Minute)

	for {
		select {
		case msg := <-msgChan:
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
			Fail("Service Metrics test timed out after 5 minutes.")
			return
		}
	}
}
