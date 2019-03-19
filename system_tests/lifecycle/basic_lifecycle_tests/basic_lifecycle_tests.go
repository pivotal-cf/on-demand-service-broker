package basic_lifecycle_tests

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

func BasicLifecycleTest(serviceType service_helpers.ServiceType, brokerInfo bosh_helpers.BrokerInfo, plan string, dopplerAddress string) {
	var (
		serviceInstanceName string
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
		serviceKeyContents := cf_helpers.GetServiceKey(serviceInstanceName, serviceKeyName)
		looksLikeAServiceKey(serviceKeyContents)
	})

	By("binding an app", func() {
		appName = "example-app" + brokerInfo.TestSuffix
		appPath := cf_helpers.GetAppPath(serviceType)
		appURL = cf_helpers.PushAndBindApp(appName, serviceInstanceName, appPath)
	})

	By("testing the app can communicate with service", func() {
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the broker emits metrics", func() {
		testMetrics(brokerInfo, plan, dopplerAddress)
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

func looksLikeAServiceKey(key string) {
	var jsonmap map[string]interface{}
	err := json.Unmarshal([]byte(key), &jsonmap)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(jsonmap)).To(BeNumerically(">", 0))
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
