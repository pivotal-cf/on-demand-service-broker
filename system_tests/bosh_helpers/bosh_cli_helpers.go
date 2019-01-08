package bosh_helpers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/gomega"
)

const (
	LongBOSHTimeout = time.Minute * 30
)

type BoshTaskOutput struct {
	Description string `json:"description"`
	State       string `json:"state"`
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

func RunErrand(deploymentName, errandName string) *gexec.Session {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", errandName)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run errand "+errandName)
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), errandName+" execution failed")
	return session
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

type BrokerInfo struct {
	URI             string
	DeploymentName  string
	ServiceOffering string
	TestSuffix      string
	BrokerPassword  string
	BrokerUsername  string
}

func DeployAndRegisterBroker(systemTestSuffix string, opsFiles ...string) BrokerInfo {
	devEnv := os.Getenv("DEV_ENV")
	if devEnv != "" {
		devEnv = "-" + devEnv
	}
	brokerPassword := uuid.New()[:6]
	deploymentName := "redis-on-demand-broker" + systemTestSuffix
	serviceOffering := "redis" + systemTestSuffix
	serviceReleaseVersion := os.Getenv("SERVICE_RELEASE_VERSION")
	serviceReleaseName := os.Getenv("SERVICE_RELEASE_NAME")
	brokerSystemDomain := os.Getenv("BROKER_SYSTEM_DOMAIN")
	bpmAvailable := os.Getenv("BPM_AVAILABLE") == "true"
	odbVersion := os.Getenv("ODB_VERSION")
	brokerURI := "redis-service-broker" + systemTestSuffix + "." + brokerSystemDomain

	fmt.Println("     --- Deploying System Test Broker ---")
	fmt.Println("")
	fmt.Printf("deploymentName        = %+v\n", deploymentName)
	fmt.Printf("serviceReleaseVersion = %+v\n", serviceReleaseVersion)
	fmt.Printf("odbVersion            = %+v\n", odbVersion)
	fmt.Printf("brokerURI             = %+v\n", brokerURI)
	fmt.Printf("brokerSystemDomain    = %+v\n", brokerSystemDomain)
	fmt.Printf("opsFiles              = %+v\n", opsFiles)
	fmt.Println("")

	varsFile := os.Getenv("BOSH_DEPLOYMENT_VARS")

	deployArguments := []string{
		"-d", deploymentName,
		"-n",
		"deploy", "./fixtures/broker_manifest.yml",
		"--vars-file", varsFile,
		"--var", "broker_uri=" + brokerURI,
		"--var", "broker_cn='*" + brokerSystemDomain + "'",
		"--var", "broker_deployment_name=" + deploymentName,
		"--var", "broker_release=on-demand-service-broker" + devEnv,
		"--var", "service_adapter_release=redis-example-service-adapter" + devEnv,
		"--var", "service_release=" + serviceReleaseName + devEnv,
		"--var", "service_release_version=" + serviceReleaseVersion,
		"--var", "broker_name=" + serviceOffering,
		"--var", "broker_route_name=redis-odb" + systemTestSuffix,
		"--var", "service_catalog_id=redis" + systemTestSuffix,
		"--var", "service_catalog_service_name=redis" + systemTestSuffix,
		"--var", "plan_id=redis-post-deploy-plan-redis" + systemTestSuffix,
		"--var", "broker_password=" + brokerPassword,
		"--var", "odb_version=" + odbVersion,
	}
	for _, opsFile := range opsFiles {
		deployArguments = append(deployArguments, []string{"--ops-file", "./fixtures/" + opsFile}...)
	}
	if bpmAvailable {
		deployArguments = append(deployArguments, []string{"--ops-file", "./fixtures/add_bpm_job.yml"}...)
	}

	if ClientCredentialsAreInVarsFile(varsFile) {
		deployArguments = append(deployArguments, []string{"--ops-file", "./fixtures/remove_cf_user_creds.yml"}...)
	}

	cmd := exec.Command("bosh", deployArguments...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh deploy command")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "deployment failed")

	Eventually(func() bool {
		return brokerRespondsOnCatalogEndpoint(brokerURI)
	}, 30*time.Second).Should(BeTrue(), "broker catalog endpoint did not come up in a reasonable time")

	RunErrand(deploymentName, "register-broker")

	return BrokerInfo{
		URI:             brokerURI,
		DeploymentName:  deploymentName,
		ServiceOffering: serviceOffering,
		TestSuffix:      systemTestSuffix,
		BrokerPassword:  brokerPassword,
		BrokerUsername:  "broker",
	}
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

func ClientCredentialsAreInVarsFile(varsFile string) bool {
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
	return test.CF.ClientCredentials.ClientID != ""
}

func DeregisterAndDeleteBroker(deploymentName string) {
	RunErrand(deploymentName, "delete-all-service-instances-and-deregister-broker")

	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "delete-deployment")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run delete deployment")
	Eventually(session, LongBOSHTimeout).Should(gexec.Exit(0), "deregistration failed")
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
