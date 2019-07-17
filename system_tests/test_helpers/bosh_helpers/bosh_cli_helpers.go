package bosh_helpers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/onsi/gomega/types"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/env_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

type BrokerDeploymentOptions struct {
	ServiceMetrics     bool
	BrokerTLS          bool
	ODBVersion         string
	AdapterVersion     string
	ODBReleaseName     string
	AdapterReleaseName string
}

type BoshTaskOutput struct {
	Description string `json:"description"`
	State       string `json:"state"`
}

type BrokerInfo struct {
	URI                string
	DeploymentName     string
	ServiceName        string
	PlanID             string
	TestSuffix         string
	BrokerPassword     string
	BrokerUsername     string
	BrokerName         string
	ServiceID          string
	BrokerSystemDomain string
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
	DeploymentName            string
	OdbReleaseTemplatesPath   string
	OdbVersion                string
	ServiceAdapterReleaseName string
	ServiceReleaseName        string
	ServiceReleaseVersion     string
	UniqueID                  string
	ServiceAdapterVersion     string
}

type EnvVars struct {
	BrokerDeploymentVarsPath  string
	BrokerSystemDomain        string
	BrokerURI                 string
	DevEnv                    string
	OdbReleaseTemplatesPath   string
	OdbVersion                string
	ServiceAdapterReleaseName string
	ServiceReleaseName        string
	ServiceReleaseVersion     string
	DeploymentName            string
}

const (
	LongBOSHTimeout = time.Minute * 45
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

func RedeployBroker(brokerDeploymentName, brokerURI string, brokerManifest bosh.BoshManifest) {
	manifestFile := manifestToFile(brokerManifest, brokerDeploymentName)

	deployODBWithManifest(brokerDeploymentName, manifestFile)

	WaitBrokerToStart(brokerURI)

	err := os.Remove(manifestFile.Name())
	Expect(err).NotTo(HaveOccurred())
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

func CopyFromVM(deploymentName, VMName, fromPath, toPath string) {
	cmd := exec.Command("bosh", "-d", deploymentName, "scp", fmt.Sprintf("%s:%s", VMName, fromPath), toPath)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run scp")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "Expected to SCP successfully")
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

func buildDeploymentArguments(systemTestSuffix string, deploymentOptions BrokerDeploymentOptions, serviceType service_helpers.ServiceType) deploymentProperties {
	envVars := getEnvVars(serviceType)

	devEnv := envVars.DevEnv
	if devEnv != "" {
		devEnv = "-" + devEnv
	}

	odbVersion := deploymentOptions.ODBVersion
	if odbVersion == "" {
		odbVersion = envVars.OdbVersion
		fmt.Printf("broker version: %s", odbVersion)
		if odbVersion == "" {
			fmt.Println("⚠ ODB version not set. Falling back to latest ⚠")
			odbVersion = "latest"
		}
	}

	serviceAdapterReleaseName := envVars.ServiceAdapterReleaseName + devEnv
	if deploymentOptions.AdapterReleaseName != "" {
		serviceAdapterReleaseName = deploymentOptions.AdapterReleaseName
	}

	serviceReleaseName := envVars.ServiceReleaseName + devEnv
	serviceReleaseVersion := envVars.ServiceReleaseVersion

	if serviceReleaseVersion == "" {
		fmt.Println("⚠ Service Release version not set. Falling back to latest available ⚠")
		serviceReleaseVersion = GetLatestReleaseVersion(serviceReleaseName)
	}
	fmt.Printf("adapter version: %s", serviceReleaseVersion)

	brokerURI := "test-service-broker" + systemTestSuffix + "." + envVars.BrokerSystemDomain
	if envVars.BrokerURI != "" {
		brokerURI = envVars.BrokerURI
	}

	deploymentName := "on-demand-broker" + systemTestSuffix
	if envVars.DeploymentName != "" {
		deploymentName = envVars.DeploymentName
	}

	serviceAdapterVersion := "latest"
	if deploymentOptions.AdapterVersion != "" {
		serviceAdapterVersion = deploymentOptions.AdapterVersion
	}

	brokerReleaseName := "on-demand-service-broker" + devEnv
	if deploymentOptions.ODBReleaseName != "" {
		brokerReleaseName = deploymentOptions.ODBReleaseName
	}
	return deploymentProperties{
		BrokerReleaseName:         brokerReleaseName,
		BrokerCN:                  "'*" + envVars.BrokerSystemDomain + "'",
		BrokerDeploymentVarsPath:  envVars.BrokerDeploymentVarsPath,
		BrokerPassword:            uuid.New()[:6],
		BrokerRoute:               "test-odb" + systemTestSuffix,
		BrokerSystemDomain:        envVars.BrokerSystemDomain,
		BrokerURI:                 brokerURI,
		BrokerUsername:            "broker",
		DeploymentName:            deploymentName,
		OdbReleaseTemplatesPath:   envVars.OdbReleaseTemplatesPath,
		OdbVersion:                odbVersion,
		ServiceAdapterReleaseName: serviceAdapterReleaseName,
		ServiceReleaseVersion:     serviceReleaseVersion,
		UniqueID:                  "odb-test" + systemTestSuffix,
		ServiceReleaseName:        serviceReleaseName,
		ServiceAdapterVersion:     serviceAdapterVersion,
	}
}

func deploy(systemTestSuffix string, deploymentOptions BrokerDeploymentOptions, serviceType service_helpers.ServiceType, deployCmdArgs ...string) BrokerInfo {
	variables := buildDeploymentArguments(systemTestSuffix, deploymentOptions, serviceType)

	odbReleaseTemplatesPath := variables.OdbReleaseTemplatesPath
	_, currentPath, _, _ := runtime.Caller(1)
	globalFixturesPath := path.Join(path.Dir(currentPath), "../../fixtures")

	baseManifest := filepath.Join(odbReleaseTemplatesPath, "base_odb_manifest.yml")
	adapterOpsFile := filepath.Join(odbReleaseTemplatesPath, "operations", serviceType.GetServiceOpsFile())

	logDeploymentProperties(variables, deployCmdArgs)

	serviceName := "service-name-" + variables.UniqueID
	planID := "plan-" + variables.UniqueID
	brokerName := "broker-" + variables.UniqueID
	serviceCatalogID := "service-id-" + variables.UniqueID
	deployArguments := []string{
		"-d", variables.DeploymentName,
		"-n",
		"deploy", baseManifest,
		"--vars-file", variables.BrokerDeploymentVarsPath,
		"--var", "broker_cn=" + variables.BrokerCN,
		"--var", "broker_deployment_name=" + variables.DeploymentName,
		"--var", "broker_name=" + brokerName,
		"--var", "broker_password=" + variables.BrokerPassword,
		"--var", "broker_release=" + variables.BrokerReleaseName,
		"--var", "broker_route_name=" + variables.BrokerRoute,
		"--var", "broker_uri=" + variables.BrokerURI,
		"--var", "broker_version=" + variables.OdbVersion,
		"--var", "plan_id=" + planID,
		"--var", "service_adapter_release=" + variables.ServiceAdapterReleaseName,
		"--var", "service_adapter_version=" + variables.ServiceAdapterVersion,
		"--var", "service_catalog_id=" + serviceCatalogID,
		"--var", "service_catalog_service_name=" + serviceName,
		"--var", "service_release=" + variables.ServiceReleaseName,
		"--var", "service_release_version=" + variables.ServiceReleaseVersion,
		"--var", "disable_ssl_cert_verification=false",
		"--var", "stemcell_alias=xenial",
		"--ops-file", adapterOpsFile,
	}

	deployArguments = append(deployArguments, deployCmdArgs...)

	if deploymentOptions.BrokerTLS {
		tlsOpsFile := filepath.Join(globalFixturesPath, "enable_broker_tls.yml")
		deployArguments = append(deployArguments, "--ops-file", tlsOpsFile, "--var", "broker_ca_credhub_path=/services/tls_ca")
	}

	if noUserCredentialsInVarsFile(variables.BrokerDeploymentVarsPath) {
		deployArguments = append(deployArguments, "--ops-file", filepath.Join(globalFixturesPath, "add_cf_uaa_client_credentials.yml"))
	}

	cmd := exec.Command("bosh", deployArguments...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "deployment failed")

	WaitBrokerToStart(variables.BrokerURI)

	return BrokerInfo{
		URI:                variables.BrokerURI,
		DeploymentName:     variables.DeploymentName,
		ServiceName:        serviceName,
		ServiceID:          serviceCatalogID,
		PlanID:             planID,
		TestSuffix:         systemTestSuffix,
		BrokerPassword:     variables.BrokerPassword,
		BrokerUsername:     variables.BrokerUsername,
		BrokerName:         brokerName,
		BrokerSystemDomain: variables.BrokerSystemDomain,
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

func UploadRelease(releasePath string) {
	cmd := exec.Command("bosh", "upload-release", releasePath)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run upload-release command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "Failed to upload release "+releasePath)
}

func GetManifest(deploymentName string) bosh.BoshManifest {
	var manifest bosh.BoshManifest
	manifestString := GetManifestString(deploymentName)

	err := yaml.Unmarshal([]byte(manifestString), &manifest)
	Expect(err).NotTo(HaveOccurred())
	return manifest
}

func GetManifestString(deploymentName string) string {
	cmd := exec.Command("bosh", "-d", deploymentName, "manifest")
	out, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return string(out)
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

func GetBrokerLogs(deploymentName string) string {
	return getLogs(deploymentName, "broker", "broker")
}

func GetTelemetryLogs(deploymentName string) string {
	return getLogs(deploymentName, "broker", "telemetry-centralizer")
}

func getLogs(deploymentName, VMName, jobName string) string {
	downloadedLogFile, err := ioutil.TempFile("/tmp", "")
	Expect(err).NotTo(HaveOccurred())
	RunOnVM(
		deploymentName,
		VMName,
		fmt.Sprintf("sudo cp /var/vcap/sys/log/%[1]s/%[1]s.stdout.log /tmp/%[1]s.log; sudo chmod 755 /tmp/%[1]s.log", jobName),
	)
	CopyFromVM(deploymentName, VMName, fmt.Sprintf("/tmp/%s.log", jobName), downloadedLogFile.Name())
	b, err := ioutil.ReadAll(downloadedLogFile)
	Expect(err).NotTo(HaveOccurred())
	return string(b)
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

func GetLatestReleaseVersion(releaseName string) string {
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

func deployODBWithManifest(brokerDeploymentName string, manifestFile *os.File) {
	cmd := exec.Command("bosh", "-d", brokerDeploymentName, "deploy", manifestFile.Name())
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "deployment failed")
}

func noUserCredentialsInVarsFile(varsFile string) bool {
	var test struct {
		CF struct {
			UserCredentials struct {
				ClientID string `yaml:"username"`
			} `yaml:"user_credentials"`
		} `yaml:"cf"`
	}
	f, err := os.Open(varsFile)
	Expect(err).NotTo(HaveOccurred())
	varsFileContents, err := ioutil.ReadAll(f)
	Expect(err).NotTo(HaveOccurred())
	err = yaml.Unmarshal(varsFileContents, &test)
	Expect(err).NotTo(HaveOccurred())
	return test.CF.UserCredentials.ClientID == ""
}

func BOSHSupportsLinksAPIForDNS() bool {
	return !getBoshVersion().LessThan(semverOf("266.16.0"))
}

func getBoshVersion() semver.Version {
	out := boshEnvironmentCommand()

	stringVersion := versionFromBOSHJson(out)

	return semverOf(stringVersion)
}

func boshEnvironmentCommand() []byte {
	cmd := exec.Command("bosh", "env", "--json")
	out, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return out
}

func versionFromBOSHJson(out []byte) string {
	var boshOutput struct {
		Tables []struct {
			Rows []struct {
				Version string `json:"version"`
			}
		}
	}

	err := json.Unmarshal(out, &boshOutput)

	Expect(err).NotTo(HaveOccurred(), "Unmarshal failed json output for tasks")
	version := boshOutput.Tables[0].Rows[0].Version

	fmt.Printf("Version of bosh is '%s'", version)

	return version
}

func semverOf(version string) semver.Version {
	splits := strings.Split(version, " ")
	Expect(len(splits)).To(BeNumerically(">", 0))
	boshVersion, err := semver.NewVersion(splits[0])

	Expect(err).NotTo(HaveOccurred())

	return *boshVersion
}

func manifestToFile(brokerManifest bosh.BoshManifest, fileName string) *os.File {
	manifestBytes, err := yaml.Marshal(brokerManifest)
	Expect(err).NotTo(HaveOccurred())

	manifestFile, err := ioutil.TempFile("", fileName+".yml")
	Expect(err).NotTo(HaveOccurred(), "failed to create temp manifest file")

	_, err = fmt.Fprint(manifestFile, string(manifestBytes))
	Expect(err).NotTo(HaveOccurred(), "failed to write to temp manifest file")

	return manifestFile
}
