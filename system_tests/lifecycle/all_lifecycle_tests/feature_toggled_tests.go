package all_lifecycle_tests

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	gouaa "github.com/cloudfoundry-community/go-uaa"
)

func FeatureToggledLifecycleTest(
	serviceType service_helpers.ServiceType,
	brokerInfo bosh_helpers.BrokerInfo,
	planName string,
	newPlanName string,
	arbitraryParams string,
	dopplerAddress string) {

	var (
		serviceInstanceName string
		serviceInstanceGUID string
		serviceKeyContents  string
		serviceKeyName      string
		appName             string
		appURL              string

		uaaClientCreateTimestamp int64
	)

	By("logging telemetry data at startup", func() {
		stdoutLogs := bosh_helpers.GetBrokerLogs(brokerInfo.DeploymentName)
		telemetryLogTotal := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances":{"total":0}}`, brokerInfo.ServiceName)
		telemetryLogPlanSmall := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances-per-plan":{"plan-id":"%s","total":0}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-small")
		telemetryLogPlanMedium := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances-per-plan":{"plan-id":"%s","total":0}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-medium")
		Expect(stdoutLogs).To(ContainSubstring(telemetryLogTotal))
		Expect(stdoutLogs).To(SatisfyAll(
			ContainSubstring(telemetryLogTotal),
			ContainSubstring(telemetryLogPlanSmall),
			ContainSubstring(telemetryLogPlanMedium),
		))
	})

	By("creating a service", func() {
		serviceInstanceName = "service" + brokerInfo.TestSuffix
		cf_helpers.CreateService(brokerInfo.ServiceName, planName, serviceInstanceName, "")
	})

	By("creating a uaa client for the SI", func() {
		serviceInstanceGUID = cf_helpers.GetServiceInstanceGUID(serviceInstanceName)
		siClient := findUAAClient(serviceInstanceGUID)
		Expect(siClient).NotTo(BeNil(), "client_id not found on UAA: "+serviceInstanceGUID)
		Expect(siClient.DisplayName).To(Equal("lifecycle_test_client"))
		Expect(siClient.RedirectURI).To(ContainElement(cf_helpers.GetDashboardURL(serviceInstanceGUID)))
		uaaClientCreateTimestamp = siClient.LastModified
	})

	By("logging telemetry data after a create-service", func() {
		stdoutLogs := bosh_helpers.GetBrokerLogs(brokerInfo.DeploymentName)

		telemetryLogTotal := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"create"},"service-instances":{"total":1}}`, brokerInfo.ServiceName)
		telemetryLogPlanSmall := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"create"},"service-instances-per-plan":{"plan-id":"%s","total":1}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-small")
		telemetryLogPlanMedium := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"create"},"service-instances-per-plan":{"plan-id":"%s","total":0}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-medium")
		Expect(stdoutLogs).To(SatisfyAll(
			ContainSubstring(telemetryLogTotal),
			ContainSubstring(telemetryLogPlanSmall),
			ContainSubstring(telemetryLogPlanMedium),
		))
	})

	By("creating a service key", func() {
		serviceKeyName = "serviceKey" + brokerInfo.TestSuffix
		cf_helpers.CreateServiceKey(serviceInstanceName, serviceKeyName)
		serviceKeyContents = cf_helpers.GetServiceKey(serviceInstanceName, serviceKeyName)

		cf_helpers.LooksLikeAServiceKey(serviceKeyContents)
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
		testMetrics(brokerInfo, planName, dopplerAddress)
	})

	By("validating the broker indicator protocol", func() {
		downloadedIndicator := downloadIndicatorFromVM(brokerInfo)

		cmd := exec.Command("indicator-verification",
			"-indicators", downloadedIndicator.Name(),
			"-authorization", cf_helpers.GetOAuthToken(),
			"-query-endpoint", "https://log-cache."+brokerInfo.BrokerSystemDomain, "-k")

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred(), "failed to run verification tool")
		Eventually(session, time.Minute).Should(gexec.Exit(0), "Indicators could not be verified")
	})

	By("testing the app can communicate with service", func() {
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating the planName for the service", func() {
		cf_helpers.UpdateServiceToPlan(serviceInstanceName, newPlanName)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating arbitrary parameters for the service", func() {
		cf_helpers.UpdateServiceWithArbitraryParams(serviceInstanceName, arbitraryParams)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("verifying that the service instance client is updated", func() {
		siClient := findUAAClient(serviceInstanceGUID)
		Expect(siClient).NotTo(BeNil(), "client_id not found on UAA: "+serviceInstanceGUID)
		Expect(uaaClientCreateTimestamp).To(BeNumerically("<", siClient.LastModified),
			"Client wasn't modified after update")
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

	By("deleting the uaa client created for the SI", func() {
		siClient := findUAAClient(serviceInstanceGUID)
		Expect(siClient).To(BeNil())
	})

	By("logging telemetry data after a delete-service", func() {
		stdoutLogs := bosh_helpers.GetBrokerLogs(brokerInfo.DeploymentName)
		// total number of instances will not decrease since we are using CF to get the count and CF is not aware of the result of delete at the point of logging.
		telemetryLogTotal := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"delete"},"service-instances":{"total":1}}`, brokerInfo.ServiceName)
		telemetryLogPlanSmall := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"delete"},"service-instances-per-plan":{"plan-id":"%s","total":0}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-small")
		telemetryLogPlanMedium := fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"instance","operation":"delete"},"service-instances-per-plan":{"plan-id":"%s","total":1}}`, brokerInfo.ServiceName, brokerInfo.PlanID+"-medium")
		Expect(stdoutLogs).To(SatisfyAll(
			ContainSubstring(telemetryLogTotal),
			ContainSubstring(telemetryLogPlanSmall),
			ContainSubstring(telemetryLogPlanMedium),
		))
	})
}

func findUAAClient(clientGUID string) *gouaa.Client {
	uaaClientID := os.Getenv("CF_CLIENT_ID")
	uaaClientSecret := os.Getenv("CF_CLIENT_SECRET")
	uaaURL := os.Getenv("CF_UAA_URL")
	api, err := gouaa.New(
		uaaURL,
		gouaa.WithClientCredentials(uaaClientID, uaaClientSecret, gouaa.JSONWebToken),
		gouaa.WithSkipSSLValidation(true),
	)
	Expect(err).ToNot(HaveOccurred())

	filter := fmt.Sprintf("client_id eq %q", clientGUID)
	siClients, _, err := api.ListClients(filter, "", gouaa.SortAscending, 1, 1)
	Expect(err).ToNot(HaveOccurred())
	if len(siClients) == 0 {
		return nil
	}
	return &siClients[0]
}

func downloadIndicatorFromVM(brokerInfo bosh_helpers.BrokerInfo) *os.File {
	downloadedIndicator, err := ioutil.TempFile("/tmp", "")
	Expect(err).NotTo(HaveOccurred())
	bosh_helpers.CopyFromVM(brokerInfo.DeploymentName, "broker", "/var/vcap/jobs/broker/config/indicators.yml", downloadedIndicator.Name())

	return downloadedIndicator
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
	bindingCredentials, err := cf_helpers.AppBindingCreds(appName, brokerInfo.ServiceName)
	Expect(err).NotTo(HaveOccurred())
	credMap, ok := bindingCredentials.(map[string]interface{})
	Expect(ok).To(BeTrue())
	credhubRef, ok := credMap["credhub-ref"].(string)
	Expect(ok).To(BeTrue(), fmt.Sprintf("unable to find credhub-ref in credentials %+v", credMap))
	Expect(credhubRef).To(ContainSubstring("/c/%s", brokerInfo.ServiceID))
}

func snakeCase(value string) string {
	return strings.ReplaceAll(value, "-", "_")
}

func testMetrics(brokerInfo bosh_helpers.BrokerInfo, planName string, dopplerAddress string) {
	brokerDeploymentName := brokerInfo.DeploymentName
	serviceOfferingName := brokerInfo.ServiceName
	Expect(dopplerAddress).NotTo(BeEmpty())

	firehoseConsumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
	defer firehoseConsumer.Close()

	msgChan, errChan := firehoseConsumer.Firehose("SystemTests-"+uuid.New(), cf_helpers.GetOAuthToken())
	timeoutChan := time.After(5 * time.Minute)
	for {
		select {
		case msg := <-msgChan:
			if msg != nil && *msg.EventType == events.Envelope_ValueMetric && strings.HasSuffix(*msg.Deployment, brokerDeploymentName) {
				if msg.ValueMetric.GetName() == fmt.Sprintf("_on_demand_broker_%s_%s_total_instances", snakeCase(serviceOfferingName), snakeCase(planName)) {
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
