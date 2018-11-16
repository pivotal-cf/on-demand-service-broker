package service_catalog_test

import (
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
)

func TestServiceCatalog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceCatalog Suite")
}

const (
	longBOSHTimeout = time.Minute * 30
)

var (
	deploymentName     string
	serviceOffering    string
	brokerPassword     string
	brokerURI          string
	brokerSystemDomain string
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	deploymentName = "redis-on-demand-broker-" + uniqueID
	serviceOffering = "redis-" + uniqueID
	brokerPassword = uuid.New()[:6]
	brokerSystemDomain := os.Getenv("BROKER_SYSTEM_DOMAIN")
	brokerURI = "redis-service-broker-" + uniqueID + "." + brokerSystemDomain
	deployAndRegisterBroker(uniqueID, deploymentName, serviceOffering)
})

var _ = AfterSuite(func() {
	deregisterAndDeleteBroker(deploymentName)
})

func deployAndRegisterBroker(uniqueID, deploymentName, serviceName string) {
	devEnv := os.Getenv("DEV_ENV")
	serviceReleaseVersion := os.Getenv("SERVICE_RELEASE_VERSION")
	brokerCACredhubName := os.Getenv("BROKER_CA_NAME")
	deployArguments := []string{
		"-d", deploymentName,
		"-n",
		"deploy", "./fixtures/broker_manifest.yml",
		"--vars-file", os.Getenv("BOSH_DEPLOYMENT_VARS"),
		"--var", "broker_ca_name='" + brokerCACredhubName + "'",
		"--var", "broker_uri=" + brokerURI,
		"--var", "broker_cn='*" + brokerSystemDomain + "'",
		"--var", "broker_deployment_name=" + deploymentName,
		"--var", "broker_release=on-demand-service-broker-" + devEnv,
		"--var", "service_adapter_release=redis-example-service-adapter-" + devEnv,
		"--var", "service_release=redis-service-" + devEnv,
		"--var", "service_release_version=" + serviceReleaseVersion,
		"--var", "broker_name=" + serviceName,
		"--var", "broker_route_name=redis-odb-" + uniqueID,
		"--var", "service_catalog_id=redis-" + uniqueID,
		"--var", "service_catalog_service_name=redis-" + uniqueID,
		"--var", "plan_id=redis-small" + uniqueID,
		"--var", "broker_password=" + brokerPassword,
	}
	cmd := exec.Command("bosh", deployArguments...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "deployment failed")

	cmd = exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", "register-broker")
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run register-broker errand")

	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "registration failed")
}

func deregisterAndDeleteBroker(deploymentName string) {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", "deregister-broker")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run deregister-broker errand")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "deregistration failed")

	cmd = exec.Command("bosh", "-n", "-d", deploymentName, "delete-deployment")
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete deployment")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "delete-deployment failed")
}
