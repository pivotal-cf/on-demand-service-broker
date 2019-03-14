package bosh_helpers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega/types"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/env_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"gopkg.in/yaml.v2"
)

type BrokerDeploymentOptions struct {
	ServiceMetrics bool
	BrokerTLS      bool
}

type BoshTaskOutput struct {
	Description string `json:"description"`
	State       string `json:"state"`
}

type BrokerInfo struct {
	URI             string
	DeploymentName  string
	ServiceOffering string
	PlanID          string
	TestSuffix      string
	BrokerPassword  string
	BrokerUsername  string
}

type deploymentProperties struct {
	BrokerCN                  string
	BrokerDeploymentVarsPath  string
	BrokerPassword            string
	BrokerReleaseName         string
	BrokerRoute               string
	BrokerSystemDomain        string
	BrokerURI                 string
	BrokerUsername            string
	ConsulRequired            string
	DeploymentName            string
	OdbReleaseTemplatesPath   string
	OdbVersion                string
	ServiceAdapterReleaseName string
	ServiceReleaseName        string
	ServiceReleaseVersion     string
	UniqueID                  string
}

type EnvVars struct {
	BrokerDeploymentVarsPath  string
	BrokerSystemDomain        string
	BrokerURI                 string
	ConsulRequired            string
	DevEnv                    string
	OdbReleaseTemplatesPath   string
	OdbVersion                string
	ServiceAdapterReleaseName string
	ServiceReleaseName        string
	ServiceReleaseVersion     string
	DeploymentName            string
}

const (
	LongBOSHTimeout = time.Minute * 30
)

func TasksForDeployment(boshServiceInstanceName string) []BoshTaskOutput {
	cmd := exec.Command("bosh", "-n", "-d", boshServiceInstanceName, "tasks", "--recent", "--json")
	stdout := gbytes.NewBuffer()
	session, err := gexec.Start(cmd, stdout, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh tasks command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "getting tasks failed")

	var boshOutput struct {
		Tables []struct {
			Rows []BoshTaskOutput
		}
	}
	err = json.Unmarshal(stdout.Contents(), &boshOutput)
	Expect(err).NotTo(HaveOccurred(), "Failed unmarshalling json output for tasks")

	return boshOutput.Tables[0].Rows
}

func RunErrand(deploymentName, errandName string, optionalMatcher ...types.GomegaMatcher) *gexec.Session {
	var matcher types.GomegaMatcher
	if optionalMatcher == nil {
		matcher = gexec.Exit(0)
	} else {
		matcher = optionalMatcher[0]
	}
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", errandName)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run errand "+errandName)
	Eventually(session, LongBOSHTimeout).Should(matcher, errandName+" execution failed")
	return session
}

func WaitBrokerToStart(brokerURI string) {
	Eventually(func() bool {
		return brokerRespondsOnCatalogEndpoint(brokerURI)
	}, 30*time.Second).Should(BeTrue(), "broker catalog endpoint did not come up in a reasonable time")
}

func VMIDForDeployment(deploymentName string) string {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "--json", "vms")
	stdout := gbytes.NewBuffer()
	session, err := gexec.Start(cmd, stdout, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh vms")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "getting VM info failed")

	var boshOutput struct {
		Tables []struct {
			Rows []struct {
				VMCID string `json:"vm_cid"`
			}
		}
	}
	err = json.Unmarshal(stdout.Contents(), &boshOutput)
	Expect(err).NotTo(HaveOccurred(), "Failed unmarshalling json output for VM info")

	return boshOutput.Tables[0].Rows[0].VMCID
}

func DeployBroker(systemTestSuffix string, deploymentOptions BrokerDeploymentOptions, serviceType service_helpers.ServiceType, opsFiles []string, deploymentArguments ...string) BrokerInfo {
	var args []string
	for _, opsFile := range opsFiles {
		args = append(args, []string{"--ops-file", "./fixtures/" + opsFile}...)
	}
	args = append(args, deploymentArguments...)
	return deploy(systemTestSuffix, deploymentOptions, serviceType, args...)
}

func DeployAndRegisterBroker(systemTestSuffix string, deploymentOptions BrokerDeploymentOptions, serviceType service_helpers.ServiceType, opsFiles []string, deploymentArguments ...string) BrokerInfo {
	brokerInfo := DeployBroker(systemTestSuffix, deploymentOptions, serviceType, opsFiles, deploymentArguments...)
	RunErrand(brokerInfo.DeploymentName, "register-broker")
	return brokerInfo
}

func RunOnVM(deploymentName, VMName, command string) {
	err := env_helpers.ValidateEnvVars(
		"BOSH_GW_HOST",
		"BOSH_GW_USER",
		"BOSH_GW_PRIVATE_KEY",
	)
	Expect(err).ToNot(HaveOccurred())
	cmd := exec.Command("bosh", "-d", deploymentName, "ssh", VMName, "-c", command)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run ssh")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "Expected to SSH successfully")
}

func Run(deploymentName string, commands ...string) {
	args := append([]string{"-d", deploymentName}, commands...)
	cmd := exec.Command("bosh", args...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "Expected to run command successfully")
}

func getEnvVars(serviceType service_helpers.ServiceType) EnvVars {
	envVars := EnvVars{}

	err := env_helpers.ValidateEnvVars(
		"BROKER_SYSTEM_DOMAIN",
		"ODB_RELEASE_TEMPLATES_PATH",
		"BOSH_DEPLOYMENT_VARS",
	)
	Expect(err).ToNot(HaveOccurred())

	envVars.BrokerDeploymentVarsPath = os.Getenv("BOSH_DEPLOYMENT_VARS")
	envVars.BrokerSystemDomain = os.Getenv("BROKER_SYSTEM_DOMAIN")
	envVars.BrokerURI = os.Getenv("BROKER_URI")
	envVars.DeploymentName = os.Getenv("BROKER_DEPLOYMENT_NAME")
	envVars.ConsulRequired = os.Getenv("CONSUL_REQUIRED")
	envVars.DevEnv = os.Getenv("DEV_ENV")
	envVars.OdbReleaseTemplatesPath = os.Getenv("ODB_RELEASE_TEMPLATES_PATH")
	envVars.OdbVersion = os.Getenv("ODB_VERSION")

	if serviceType == service_helpers.Redis {
		err := env_helpers.ValidateEnvVars(
			"REDIS_SERVICE_ADAPTER_RELEASE_NAME", "REDIS_SERVICE_RELEASE_NAME",
		)
		Expect(err).ToNot(HaveOccurred())
		envVars.ServiceAdapterReleaseName = os.Getenv("REDIS_SERVICE_ADAPTER_RELEASE_NAME")
		envVars.ServiceReleaseName = os.Getenv("REDIS_SERVICE_RELEASE_NAME")
		envVars.ServiceReleaseVersion = os.Getenv("REDIS_SERVICE_RELEASE_VERSION")
	} else {
		err := env_helpers.ValidateEnvVars(
			"KAFKA_SERVICE_ADAPTER_RELEASE_NAME", "KAFKA_SERVICE_RELEASE_NAME",
		)
		Expect(err).ToNot(HaveOccurred())
		envVars.ServiceAdapterReleaseName = os.Getenv("KAFKA_SERVICE_ADAPTER_RELEASE_NAME")
		envVars.ServiceReleaseName = os.Getenv("KAFKA_SERVICE_RELEASE_NAME")
		envVars.ServiceReleaseVersion = os.Getenv("KAFKA_SERVICE_RELEASE_VERSION")
	}
	return envVars
}

func buildDeploymentArguments(systemTestSuffix string, serviceType service_helpers.ServiceType) deploymentProperties {
	envVars := getEnvVars(serviceType)

	devEnv := envVars.DevEnv
	if devEnv != "" {
		devEnv = "-" + devEnv
	}

	odbVersion := envVars.OdbVersion
	if odbVersion == "" {
		fmt.Println("⚠ ODB version not set. Falling back to latest ⚠")
		odbVersion = "latest"
	}

	serviceAdapterReleaseName := envVars.ServiceAdapterReleaseName + devEnv
	serviceReleaseName := envVars.ServiceReleaseName + devEnv
	serviceReleaseVersion := envVars.ServiceReleaseVersion
	if serviceReleaseVersion == "" {
		fmt.Println("⚠ Service Release version not set. Falling back to latest available ⚠")
		serviceReleaseVersion = getLatestServiceReleaseVersion(serviceReleaseName)
	}

	brokerURI := "test-service-broker" + systemTestSuffix + "." + envVars.BrokerSystemDomain
	if envVars.BrokerURI != "" {
		brokerURI = envVars.BrokerURI
	}

	deploymentName := "on-demand-broker" + systemTestSuffix
	if envVars.DeploymentName != "" {
		deploymentName = envVars.DeploymentName
	}

	return deploymentProperties{
		BrokerReleaseName:         "on-demand-service-broker" + devEnv,
		BrokerCN:                  "'*" + envVars.BrokerSystemDomain + "'",
		BrokerDeploymentVarsPath:  envVars.BrokerDeploymentVarsPath,
		BrokerPassword:            uuid.New()[:6],
		BrokerRoute:               "test-odb" + systemTestSuffix,
		BrokerSystemDomain:        envVars.BrokerSystemDomain,
		BrokerURI:                 brokerURI,
		BrokerUsername:            "broker",
		ConsulRequired:            envVars.ConsulRequired,
		DeploymentName:            deploymentName,
		OdbReleaseTemplatesPath:   envVars.OdbReleaseTemplatesPath,
		OdbVersion:                odbVersion,
		ServiceAdapterReleaseName: serviceAdapterReleaseName,
		ServiceReleaseVersion:     serviceReleaseVersion,
		UniqueID:                  "odb-test" + systemTestSuffix,
		ServiceReleaseName:        serviceReleaseName,
	}
}

func deploy(systemTestSuffix string, deploymentOptions BrokerDeploymentOptions, serviceType service_helpers.ServiceType, deployCmdArgs ...string) BrokerInfo {
	variables := buildDeploymentArguments(systemTestSuffix, serviceType)

	odbReleaseTemplatesPath := variables.OdbReleaseTemplatesPath
	baseManifest := filepath.Join(odbReleaseTemplatesPath, "base_odb_manifest.yml")
	adapterOpsFile := filepath.Join(odbReleaseTemplatesPath, "operations", serviceType.GetServiceOpsFile())

	logDeploymentProperties(variables, deployCmdArgs)

	deployArguments := []string{
		"-d", variables.DeploymentName,
		"-n",
		"deploy", baseManifest,
		"--vars-file", variables.BrokerDeploymentVarsPath,
		"--var", "broker_cn=" + variables.BrokerCN,
		"--var", "broker_deployment_name=" + variables.DeploymentName,
		"--var", "broker_name=" + variables.UniqueID,
		"--var", "broker_password=" + variables.BrokerPassword,
		"--var", "broker_release=" + variables.BrokerReleaseName,
		"--var", "broker_route_name=" + variables.BrokerRoute,
		"--var", "broker_uri=" + variables.BrokerURI,
		"--var", "broker_version=" + variables.OdbVersion,
		"--var", "plan_id=" + variables.UniqueID,
		"--var", "service_adapter_release=" + variables.ServiceAdapterReleaseName,
		"--var", "service_adapter_version=latest",
		"--var", "service_catalog_id=" + variables.UniqueID,
		"--var", "service_catalog_service_name=" + variables.UniqueID,
		"--var", "service_release=" + variables.ServiceReleaseName,
		"--var", "service_release_version=" + variables.ServiceReleaseVersion,
		"--var", "disable_ssl_cert_verification=false",
		"--var", "stemcell_alias=xenial",

		"--ops-file", adapterOpsFile,
	}

	deployArguments = append(deployArguments, deployCmdArgs...)

	if deploymentOptions.BrokerTLS {
		tlsOpsFile := filepath.Join(odbReleaseTemplatesPath, "operations", "enable_broker_tls.yml")
		deployArguments = append(deployArguments, "--ops-file", tlsOpsFile, "--var", "broker_ca_credhub_path=/services/tls_ca")
	}

	consulRequired := variables.ConsulRequired == "true"
	if consulRequired {
		deployArguments = append(deployArguments, "--ops-file", filepath.Join(odbReleaseTemplatesPath, "operations", "add_consul.yml"))
	}

	if noClientCredentialsInVarsFile(variables.BrokerDeploymentVarsPath) {
		deployArguments = append(deployArguments, "--ops-file", filepath.Join(odbReleaseTemplatesPath, "operations", "cf_uaa_user.yml"))
	}

	cmd := exec.Command("bosh", deployArguments...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "deployment failed")

	WaitBrokerToStart(variables.BrokerURI)

	return BrokerInfo{
		URI:             variables.BrokerURI,
		DeploymentName:  variables.DeploymentName,
		ServiceOffering: variables.UniqueID,
		PlanID:          variables.UniqueID,
		TestSuffix:      systemTestSuffix,
		BrokerPassword:  variables.BrokerPassword,
		BrokerUsername:  variables.BrokerUsername,
	}
}

func logDeploymentProperties(variables deploymentProperties, deployCmdArgs []string) {
	fmt.Println("     --- Deploying System Test Broker ---")
	fmt.Println("")
	fmt.Printf("deploymentName        = %+v\n", variables.DeploymentName)
	fmt.Printf("serviceReleaseVersion = %+v\n", variables.ServiceReleaseVersion)
	fmt.Printf("odbVersion            = %+v\n", variables.OdbVersion)
	fmt.Printf("brokerURI             = %+v\n", variables.BrokerURI)
	fmt.Printf("brokerSystemDomain    = %+v\n", variables.BrokerSystemDomain)
	fmt.Printf("deployCmdArgs              = %+v\n", deployCmdArgs)
	fmt.Println("")
}

func GetBOSHConfig(configType, configName string) (string, error) {
	args := []string{
		"config",
		"--type", configType,
		"--name", configName,
	}
	cmd := exec.Command("bosh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func DeleteBOSHConfig(configType, configName string) error {
	args := []string{
		"delete-config",
		"--type", configType,
		"--name", configName,
		"-n",
	}
	cmd := exec.Command("bosh", args...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func DeleteDeployment(deploymentName string) {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "delete-deployment")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete deployment")
	Eventually(session, LongBOSHTimeout).Should(
		gexec.Exit(0),
		"delete-deployment failed",
	)
}

func DeregisterAndDeleteBroker(deploymentName string) {
	RunErrand(deploymentName, "delete-all-service-instances-and-deregister-broker")
	DeleteDeployment(deploymentName)
}

func DeregisterAndDeleteBrokerSilently(deploymentName string) {
	RunErrand(deploymentName,
		"delete-all-service-instances-and-deregister-broker",
		Or(gexec.Exit(0), gexec.Exit(1)),
	)
	DeleteDeployment(deploymentName)
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

func getLatestServiceReleaseVersion(releaseName string) string {
	releasesOutput := gbytes.NewBuffer()
	cmd := exec.Command("bosh", "releases", "--json")
	session, err := gexec.Start(cmd, releasesOutput, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete deployment")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "failed to retrieve bosh releases")

	var boshOutput struct {
		Tables []struct {
			Rows []struct {
				Name    string
				Version string
			}
		}
	}
	Expect(json.Unmarshal(releasesOutput.Contents(), &boshOutput)).NotTo(HaveOccurred())
	Expect(boshOutput.Tables).To(HaveLen(1))
	for _, release := range boshOutput.Tables[0].Rows {
		if release.Name == releaseName {
			return strings.TrimRight(release.Version, "*")
		}
	}

	Fail("Could not find version for " + releaseName + " release")
	return ""
}

func noClientCredentialsInVarsFile(varsFile string) bool {
	var test struct {
		CF struct {
			ClientCredentials struct {
				ClientID string `yaml:"client_id"`
			} `yaml:"client_credentials"`
		} `yaml:"cf"`
	}
	f, err := os.Open(varsFile)
	Expect(err).NotTo(HaveOccurred())
	varsFileContents, err := ioutil.ReadAll(f)
	Expect(err).NotTo(HaveOccurred())
	err = yaml.Unmarshal(varsFileContents, &test)
	Expect(err).NotTo(HaveOccurred())
	return test.CF.ClientCredentials.ClientID == ""
}
