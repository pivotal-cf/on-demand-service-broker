package feature_flags_test

import (
	"testing"

	"os"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	yaml "gopkg.in/yaml.v2"
)

var (
	boshClient               *bosh_helpers.BoshHelperClient
	serviceOffering          string
	brokerName               string
	brokerBoshDeploymentName string
	cfAdminUsername          string
	cfAdminPassword          string
	cfSpaceDeveloperUsername string
	cfSpaceDeveloperPassword string
	cfOrg                    string
	cfSpace                  string

	originalBrokerManifest *bosh.BoshManifest
)

func TestFeatureFlags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureFlags Suite")
}

var _ = BeforeSuite(func() {
	serviceOffering = envMustHave("SERVICE_OFFERING_NAME")
	brokerName = envMustHave("BROKER_NAME")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")

	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")
	disableTLSVerification := boshCACert == ""
	uaaURL := os.Getenv("UAA_URL")

	cfAdminUsername = envMustHave("CF_USERNAME")
	cfAdminPassword = envMustHave("CF_PASSWORD")
	cfSpaceDeveloperUsername = uuid.New()[:8]
	cfSpaceDeveloperPassword = uuid.New()[:8]
	cfOrg = envMustHave("CF_ORG")
	cfSpace = envMustHave("CF_SPACE")

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, disableTLSVerification)
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}
	SetDefaultEventuallyTimeout(cf.CfTimeout)

	originalBrokerManifest = boshClient.GetManifest(brokerBoshDeploymentName)
	newManifest := modifyBrokerManifest(copyOf(*originalBrokerManifest))
	boshClient.DeployODB(newManifest)
	boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")
})

var _ = AfterSuite(func() {
	boshClient.RunErrand(brokerBoshDeploymentName, "delete-all-service-instances-and-deregister-broker", []string{}, "")
	boshClient.DeployODB(*originalBrokerManifest)
	boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")
})

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}

func modifyBrokerManifest(manifest bosh.BoshManifest) bosh.BoshManifest {
	var brokerJob bosh.Job
	for _, ig := range manifest.InstanceGroups {
		if ig.Name == "broker" {
			for _, job := range ig.Jobs {
				if job.Name == "broker" {
					brokerJob = job
				}
			}
		}
	}
	brokerJob.Properties["expose_operational_errors"] = true
	brokerJob.Properties["disable_ssl_cert_verification"] = true
	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})
	plan := serviceCatalog["plans"].([]interface{})[0].(map[interface{}]interface{})
	plan["name"] = "invalid-vm-type"
	plan["plan_id"] = "invalid-vm-type-id" + uuid.New()[:7]
	instanceGroup := plan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})
	instanceGroup["vm_type"] = "invalid-vm-type"

	return manifest
}

func copyOf(obj bosh.BoshManifest) bosh.BoshManifest {
	manifestJson, err := yaml.Marshal(obj)
	Expect(err).ToNot(HaveOccurred())
	var m bosh.BoshManifest
	Expect(yaml.Unmarshal(manifestJson, &m)).ToNot(HaveOccurred())
	return m
}
