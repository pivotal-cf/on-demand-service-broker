package recreate_all_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

func TestRecreateAll(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecreateAll Suite")
}

const (
	longBOSHTimeout = time.Minute * 30
)

var (
	deploymentName      string
	serviceOffering     string
	serviceInstanceName string
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	systemTestSuffix := "-recreate-" + uniqueID
	// test deployments must begin with "redis-on-demand-broker-" to be automatically cleaned up
	deploymentName = "redis-on-demand-broker" + systemTestSuffix
	serviceOffering = "redis" + systemTestSuffix
	serviceInstanceName = "service" + systemTestSuffix
	deployAndRegisterBroker(systemTestSuffix, deploymentName, serviceOffering)
	cf.CreateService(serviceOffering, "redis-with-post-deploy", serviceInstanceName, "")
})

var _ = AfterSuite(func() {
	deregisterAndDeleteBroker(deploymentName)
})

func deployAndRegisterBroker(systemTestSuffix, deploymentName, serviceName string) {
	devEnv := os.Getenv("DEV_ENV")
	if devEnv != "" {
		devEnv = "-" + devEnv
	}
	serviceReleaseVersion := os.Getenv("SERVICE_RELEASE_VERSION")
	serviceReleaseName := os.Getenv("SERVICE_RELEASE_NAME")
	brokerSystemDomain := os.Getenv("BROKER_SYSTEM_DOMAIN")
	bpmAvailable := os.Getenv("BPM_AVAILABLE") == "true"
	odbVersion := os.Getenv("ODB_VERSION")
	brokerURI := "redis-service-broker" + systemTestSuffix + "." + brokerSystemDomain

	fmt.Println("--- System Test Details ---")
	fmt.Println("")
	fmt.Printf("deploymentName        = %+v\n", deploymentName)
	fmt.Printf("serviceReleaseVersion = %+v\n", serviceReleaseVersion)
	fmt.Printf("odbVersion            = %+v\n", odbVersion)
	fmt.Printf("brokerURI             = %+v\n", brokerURI)
	fmt.Printf("brokerSystemDomain    = %+v\n", brokerSystemDomain)
	fmt.Println("")

	deployArguments := []string{
		"-d", deploymentName,
		"-n",
		"deploy", "./fixtures/broker_manifest.yml",
		"--vars-file", os.Getenv("BOSH_DEPLOYMENT_VARS"),
		"--var", "broker_uri=" + brokerURI,
		"--var", "broker_cn='*" + brokerSystemDomain + "'",
		"--var", "broker_deployment_name=" + deploymentName,
		"--var", "broker_release=on-demand-service-broker" + devEnv,
		"--var", "service_adapter_release=redis-example-service-adapter" + devEnv,
		"--var", "service_release=" + serviceReleaseName + devEnv,
		"--var", "service_release_version=" + serviceReleaseVersion,
		"--var", "broker_name=" + serviceName,
		"--var", "broker_route_name=redis-odb" + systemTestSuffix,
		"--var", "service_catalog_id=redis" + systemTestSuffix,
		"--var", "service_catalog_service_name=redis" + systemTestSuffix,
		"--var", "plan_id=redis-post-deploy-plan-redis" + systemTestSuffix,
		"--var", "odb_version=" + odbVersion,
	}
	if bpmAvailable {
		deployArguments = append(deployArguments, []string{"--ops-file", "./fixtures/add_bpm_job.yml"}...)
	}

	cmd := exec.Command("bosh", deployArguments...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "deployment failed")

	Eventually(func() bool {
		return brokerRespondsOnCatalogEndpoint(brokerURI)
	}, 30*time.Second).Should(BeTrue(), "broker catalog endpoint did not come up in reasonable time")

	cmd = exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", "register-broker")
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run register-broker errand")

	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "registration failed")
}

func deregisterAndDeleteBroker(deploymentName string) {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", "delete-all-service-instances-and-deregister-broker")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete-all errand")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "deregistration failed")

	cmd = exec.Command("bosh", "-n", "-d", deploymentName, "delete-deployment")
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete deployment")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "deregistration failed")
}

func brokerRespondsOnCatalogEndpoint(brokerURI string) bool {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := http.Client{
		Transport: transport,
	}
	res, err := client.Get("https://" + brokerURI + "/v2/catalog")
	if err != nil {
		return false
	}

	return res.StatusCode == http.StatusUnauthorized
}