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

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/env_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"gopkg.in/yaml.v2"
)

const (
	LongBOSHTimeout = time.Minute * 30
)

type BoshTaskOutput struct {
	Description string `json:"description"`
	State       string `json:"state"`
}

type BrokerInfo struct {
	URI             string
	DeploymentName  string
	ServiceOffering string
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
	DisableBPM                bool
	OdbReleaseTemplatesPath   string
	OdbVersion                string
	ServiceAdapterReleaseName string
	ServiceReleaseName        string
	ServiceReleaseVersion     string
	UniqueID                  string
}

type EnvVars struct {
	DevEnv                   string
	BrokerSystemDomain       string
	DisableBPM               bool
	ConsulRequired           string
	OdbVersion               string
	ServiceReleaseName       string
	ServiceReleaseVersion    string
	OdbReleaseTemplatesPath  string
	BrokerDeploymentVarsPath string
}

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

func DeployBroker(systemTestSuffix string, opsFiles []string, deploymentArguments ...string) BrokerInfo {
	var args []string
	for _, opsFile := range opsFiles {
		args = append(args, []string{"--ops-file", "./fixtures/" + opsFile}...)
	}
	args = append(args, deploymentArguments...)
	return deploy(systemTestSuffix, args...)
}

func DeployAndRegisterBroker(systemTestSuffix string, opsFiles []string, deploymentArguments ...string) BrokerInfo {
	brokerInfo := DeployBroker(systemTestSuffix, opsFiles, deploymentArguments...)
	RunErrand(brokerInfo.DeploymentName, "register-broker")
	return brokerInfo
}

func RunOnVM(deploymentName, VMName, command string) {
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

func getEnvVars() EnvVars {
	envVars := EnvVars{}

	err := env_helpers.ValidateEnvVars(
		"SERVICE_RELEASE_NAME",
		"BROKER_SYSTEM_DOMAIN",
		"ODB_RELEASE_TEMPLATES_PATH",
		"BOSH_DEPLOYMENT_VARS",
	)
	Expect(err).ToNot(HaveOccurred())

	envVars.DevEnv = os.Getenv("DEV_ENV")
	envVars.BrokerSystemDomain = os.Getenv("BROKER_SYSTEM_DOMAIN")
	envVars.DisableBPM = os.Getenv("DISABLE_BPM") == "true"
	envVars.ConsulRequired = os.Getenv("CONSUL_REQUIRED")
	envVars.OdbVersion = os.Getenv("ODB_VERSION")
	envVars.ServiceReleaseName = os.Getenv("SERVICE_RELEASE_NAME")
	envVars.ServiceReleaseVersion = os.Getenv("SERVICE_RELEASE_VERSION")
	envVars.OdbReleaseTemplatesPath = os.Getenv("ODB_RELEASE_TEMPLATES_PATH")
	envVars.BrokerDeploymentVarsPath = os.Getenv("BOSH_DEPLOYMENT_VARS")
	return envVars
}

func buildDeploymentArguments(systemTestSuffix string) deploymentProperties {
	envVars := getEnvVars()

	devEnv := envVars.DevEnv
	if devEnv != "" {
		devEnv = "-" + devEnv
	}

	odbVersion := envVars.OdbVersion
	if odbVersion == "" {
		fmt.Println("⚠ ODB version not set. Falling back to latest ⚠")
		odbVersion = "latest"
	}

	serviceReleaseName := envVars.ServiceReleaseName + devEnv

	serviceReleaseVersion := envVars.ServiceReleaseVersion
	if serviceReleaseVersion == "" {
		fmt.Println("⚠ Service Release version not set. Falling back to latest available ⚠")
		serviceReleaseVersion = getLatestServiceReleaseVersion(serviceReleaseName)
	}

	return deploymentProperties{
		BrokerReleaseName:         "on-demand-service-broker" + devEnv,
		BrokerCN:                  "'*" + envVars.BrokerSystemDomain + "'",
		BrokerDeploymentVarsPath:  envVars.BrokerDeploymentVarsPath,
		BrokerPassword:            uuid.New()[:6],
		BrokerRoute:               "redis-odb" + systemTestSuffix,
		BrokerSystemDomain:        envVars.BrokerSystemDomain,
		BrokerURI:                 "redis-service-broker" + systemTestSuffix + "." + envVars.BrokerSystemDomain,
		BrokerUsername:            "broker",
		ConsulRequired:            envVars.ConsulRequired,
		DeploymentName:            "redis-on-demand-broker" + systemTestSuffix,
		DisableBPM:                envVars.DisableBPM,
		OdbReleaseTemplatesPath:   envVars.OdbReleaseTemplatesPath,
		OdbVersion:                odbVersion,
		ServiceAdapterReleaseName: "redis-example-service-adapter" + devEnv,
		ServiceReleaseVersion:     serviceReleaseVersion,
		UniqueID:                  "redis" + systemTestSuffix,
		ServiceReleaseName:        serviceReleaseName,
	}
}

func deploy(systemTestSuffix string, deployCmdArgs ...string) BrokerInfo {

	variables := buildDeploymentArguments(systemTestSuffix)

	odbReleaseTemplatesPath := variables.OdbReleaseTemplatesPath
	baseManifest := filepath.Join(odbReleaseTemplatesPath, "base_odb_manifest.yml")
	redisAdapterOpsFile := filepath.Join(odbReleaseTemplatesPath, "operations", "redis.yml")

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
		"--var", "instance_groups_vm_extensions=[public_ip]",
		"--var", "disable_ssl_cert_verification=false",
		"--var", "stemcell_alias=xenial",

		"--ops-file", redisAdapterOpsFile,
	}
	deployArguments = append(deployArguments, deployCmdArgs...)

	if variables.DisableBPM {
		deployArguments = append(deployArguments, []string{"--ops-file", filepath.Join(odbReleaseTemplatesPath, "operations", "remove_bpm.yml")}...)
	}

	consulRequired := variables.ConsulRequired == "true"
	if consulRequired {
		deployArguments = append(deployArguments, []string{"--ops-file", filepath.Join(odbReleaseTemplatesPath, "operations", "add_consul.yml")}...)
	}

	if noClientCredentialsInVarsFile(variables.BrokerDeploymentVarsPath) {
		deployArguments = append(deployArguments, []string{"--ops-file", filepath.Join(odbReleaseTemplatesPath, "operations", "cf_uaa_user.yml")}...)
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
