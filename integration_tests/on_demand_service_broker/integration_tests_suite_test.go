// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/on_demand_service_broker/mock"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

var (
	adapter mock.Adapter

	brokerPort     int
	brokerUsername = "broker-username"
	brokerPassword = "a-very-strong-password"
	startUpBanner  = false

	boshUsername     = "boshUsername"
	boshPassword     = "boshPassword"
	boshClientID     = "boshClientID"
	boshClientSecret = "boshClientSecret"

	cfUaaClientID     = "cfAdminUsername"
	cfUaaClientSecret = "cfAdminPassword"

	serviceReleaseName    = "bosh-release-that-provides-job"
	serviceReleaseVersion = "0+dev.42"

	stemcellOS      = "ubuntu-trusty"
	stemcellVersion = "1234"

	serviceID                          = "service-id"
	serviceDescription                 = "the finest service available to humanity"
	serviceBindable                    = false
	servicePlanUpdatable               = true
	serviceMetadataDisplayName         = "service-name"
	serviceMetadataImageURL            = "service-image-url"
	serviceMetaDataLongDescription     = "serviceMetaDataLongDescription"
	serviceMetaDataProviderDisplayName = "serviceMetaDataProviderDisplayName"
	serviceMetaDataDocumentationURL    = "serviceMetaDataDocumentationURL"
	serviceMetaDataSupportURL          = "serviceMetaDataSupportURL"
	serviceMetaDataShareable           = true
	serviceTags                        = []string{"a", "b"}

	dedicatedPlanName         = "dedicated-plan-name"
	dedicatedPlanDisplayName  = "dedicated-plan-display-name"
	dedicatedPlanDescription  = "dedicatedPlanDescription"
	dedicatedPlanCostAmount   = map[string]float64{"usd": 99.0, "eur": 49.0}
	dedicatedPlanCostUnit     = "MONTHLY"
	dedicatedPlanVMType       = "dedicated-plan-vm"
	dedicatedPlanVMExtensions = []string{"what", "an", "extension"}
	dedicatedPlanDisk         = "dedicated-plan-disk"
	dedicatedPlanInstances    = 1
	dedicatedPlanQuota        = 1
	dedicatedPlanNetworks     = []string{"net1"}
	dedicatedPlanAZs          = []string{"az1"}
	dedicatedPlanBullets      = []string{"bullet one", "bullet two", "bullet three"}
	dedicatedPlanUpdateBlock  = &serviceadapter.Update{
		Canaries:        1,
		MaxInFlight:     10,
		CanaryWatchTime: "1000-30000",
		UpdateWatchTime: "1000-30000",
		Serial:          booleanPointer(false),
	}

	highMemoryPlanID           = "high-memory-plan-id"
	highMemoryPlanName         = "high-memory-plan-name"
	highMemoryPlanDescription  = "highMemoryPlanDescription"
	highMemoryPlanVMType       = "high-memory-plan-vm"
	highMemoryPlanVMExtensions = []string{"even", "more", "memory"}
	highMemoryPlanDisplayName  = "dedicated-plan-display-name"
	highMemoryPlanBullets      = []string{"bullet one", "bullet two", "bullet three"}
	highMemoryPlanInstances    = 27
	highMemoryPlanNetworks     = []string{"high1", "high2"}
	highMemoryPlanAZs          = []string{"az1", "az2"}

	spaceGUID        = "space-guid"
	organizationGUID = "organizationGuid"
)

const (
	serviceName     = "service-name"
	dedicatedPlanID = "dedicated-plan-id"
)

var (
	brokerBinPath      string
	serviceAdapterPath string
	tempDirPath        string
)

type binPaths struct {
	Broker  string
	Adapter string
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		broker, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/on-demand-service-broker")
		Expect(err).NotTo(HaveOccurred())

		adapter, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/integration_tests/on_demand_service_broker/mock/adapter")
		Expect(err).NotTo(HaveOccurred())

		compiledBinaries, err := json.Marshal(binPaths{
			Broker:  broker,
			Adapter: adapter,
		})
		Expect(err).NotTo(HaveOccurred())

		return compiledBinaries
	},
	func(fromFirstNode []byte) {
		var compiledBinaries binPaths
		Expect(json.Unmarshal(fromFirstNode, &compiledBinaries)).To(Succeed())
		brokerBinPath = compiledBinaries.Broker
		serviceAdapterPath = compiledBinaries.Adapter

		brokerPort = 37890 + GinkgoParallelNode()

		var err error
		tempDirPath, err = ioutil.TempDir("", fmt.Sprintf("broker-integration-tests-%d", GinkgoParallelNode()))
		Expect(err).ToNot(HaveOccurred())
	},
)

var _ = SynchronizedAfterSuite(func() {
	Expect(os.RemoveAll(tempDirPath)).To(Succeed())
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	adapter = mock.Adapter{}
	adapter.New()
})

var _ = AfterEach(func() {
	adapter.Cleanup()
})

func TestIntegrationTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Tests Suite")
}

func startBrokerWithPassingStartupChecks(
	conf config.Config,
	cfAPI *mockhttp.Server,
	boshDirector *mockbosh.MockBOSH,
) *gexec.Session {
	cfAPI.VerifyAndMock(
		mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
		mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
	)
	boshDirector.VerifyAndMock(
		mockbosh.Info().RespondsWithSufficientVersionForLifecycleErrands(boshDirector.UAAURL),
		mockbosh.Info().RespondsWithSufficientVersionForLifecycleErrands(boshDirector.UAAURL),
	)
	return startBroker(conf)
}

func startBroker(conf config.Config) *gexec.Session {
	session := startBrokerWithoutPortCheck(conf)
	Eventually(dialBroker).Should(BeTrue())
	return session
}

func startBrokerWithoutPortCheck(conf config.Config) *gexec.Session {
	linuxCompatibleProcessName := "on-demand-servi"
	killCmd := exec.Command("pkill", "-9", linuxCompatibleProcessName)
	killSession, err := gexec.Start(killCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	killSession.Wait()

	configContents, err := yaml.Marshal(conf)
	Expect(err).NotTo(HaveOccurred())

	testConfigFilePath := filepath.Join(tempDirPath, "broker.yml")
	Expect(ioutil.WriteFile(testConfigFilePath, configContents, 0644)).To(Succeed())

	cmd := exec.Command(brokerBinPath, "-configFilePath", testConfigFilePath)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

func dialBroker() bool {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", brokerPort))
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

func basicAuthBrokerRequest(req *http.Request) *http.Request {
	req.SetBasicAuth(brokerUsername, brokerPassword)
	return req
}

func provisionInstance(instanceID, planID string, arbitraryParams map[string]interface{}) *http.Response {
	instance, err := provisionInstanceWithAsyncFlag(instanceID, planID, arbitraryParams, true)
	Expect(err).NotTo(HaveOccurred())
	return instance
}

func provisionInstanceSynchronously(instanceID, planID string, arbitraryParams map[string]interface{}) *http.Response {
	instance, err := provisionInstanceWithAsyncFlag(instanceID, planID, arbitraryParams, false)
	Expect(err).NotTo(HaveOccurred())
	return instance
}

func deprovisionInstance(instanceID string, asyncAllowed bool) *http.Response {
	deprovisionReq, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("http://localhost:%d/v2/service_instances/%s?accepts_incomplete=%t", brokerPort, instanceID, asyncAllowed), bytes.NewReader([]byte{}))
	Expect(err).ToNot(HaveOccurred())
	deprovisionReq = basicAuthBrokerRequest(deprovisionReq)
	deprovisionResponse, err := http.DefaultClient.Do(deprovisionReq)
	Expect(err).ToNot(HaveOccurred())
	return deprovisionResponse
}

func provisionInstanceWithAsyncFlag(instanceID, planID string, arbitraryParams map[string]interface{}, async bool) (*http.Response, error) {
	reqBody := map[string]interface{}{
		"plan_id":           planID,
		"space_guid":        spaceGUID,
		"organization_guid": organizationGUID,
		"parameters":        arbitraryParams,
		"service_id":        serviceID,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return &http.Response{}, err
	}

	asyncFlag := "true"
	if !async {
		asyncFlag = "false"
	}

	provisionReq, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://localhost:%d/v2/service_instances/%s?accepts_incomplete="+asyncFlag, brokerPort, instanceID),
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return &http.Response{}, err
	}
	provisionReq = basicAuthBrokerRequest(provisionReq)

	return http.DefaultClient.Do(provisionReq)
}

func lastOperationForInstance(instanceID string, operationData broker.OperationData) *http.Response {
	lastOperationURL := fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/last_operation", brokerPort, instanceID)
	if operationData != (broker.OperationData{}) {
		operationDataBytes, err := json.Marshal(operationData)
		Expect(err).NotTo(HaveOccurred())
		lastOperationURL = fmt.Sprintf("%s?operation=%s", lastOperationURL, url.QueryEscape(string(operationDataBytes)))
	}
	req, err := http.NewRequest(
		"GET",
		lastOperationURL,
		nil,
	)
	Expect(err).NotTo(HaveOccurred())
	req = basicAuthBrokerRequest(req)
	lastOperationResponse, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return lastOperationResponse
}

func defaultBrokerConfig(boshURL, uaaURL, cfURL, cfUAAURL string) config.Config {
	return config.Config{
		Broker: config.Broker{
			Port:                brokerPort,
			Username:            brokerUsername,
			Password:            brokerPassword,
			StartUpBanner:       startUpBanner,
			ShutdownTimeoutSecs: 2,
		},
		Bosh: config.Bosh{
			URL: boshURL,
			Authentication: config.BOSHAuthentication{
				UAA: config.BOSHUAAAuthentication{ID: boshClientID, Secret: boshClientSecret},
			},
		},
		CF: config.CF{
			URL: cfURL,
			Authentication: config.UAAAuthentication{
				URL: cfUAAURL,
				ClientCredentials: config.ClientCredentials{
					ID:     cfUaaClientID,
					Secret: cfUaaClientSecret,
				},
			},
		},
		ServiceAdapter: config.ServiceAdapter{
			Path: serviceAdapterPath,
		},
		ServiceDeployment: config.ServiceDeployment{
			Releases: serviceadapter.ServiceReleases{
				{
					Name:    serviceReleaseName,
					Version: serviceReleaseVersion,
					Jobs:    []string{"job-name"},
				},
			},
			Stemcell: serviceadapter.Stemcell{
				OS:      stemcellOS,
				Version: stemcellVersion,
			},
		},
		ServiceCatalog: config.ServiceOffering{
			ID:            serviceID,
			Name:          serviceName,
			Description:   serviceDescription,
			Bindable:      serviceBindable,
			PlanUpdatable: servicePlanUpdatable,
			Metadata: config.ServiceMetadata{
				DisplayName:         serviceMetadataDisplayName,
				ImageURL:            serviceMetadataImageURL,
				LongDescription:     serviceMetaDataLongDescription,
				ProviderDisplayName: serviceMetaDataProviderDisplayName,
				DocumentationURL:    serviceMetaDataDocumentationURL,
				SupportURL:          serviceMetaDataSupportURL,
				Shareable:           serviceMetaDataShareable,
			},
			DashboardClient: &config.DashboardClient{
				ID:          "client-id-1",
				Secret:      "secret-1",
				RedirectUri: "https://dashboard.url",
			},
			Tags:             serviceTags,
			GlobalProperties: serviceadapter.Properties{"global_property": "global_value"},
			GlobalQuotas:     config.Quotas{},
			Plans: []config.Plan{
				{
					Name:        dedicatedPlanName,
					ID:          dedicatedPlanID,
					Description: dedicatedPlanDescription,
					Free:        booleanPointer(true),
					Bindable:    booleanPointer(true),
					Update:      dedicatedPlanUpdateBlock,
					Metadata: config.PlanMetadata{
						DisplayName: dedicatedPlanDisplayName,
						Bullets:     dedicatedPlanBullets,
						Costs: []config.PlanCost{
							{
								Amount: dedicatedPlanCostAmount,
								Unit:   dedicatedPlanCostUnit,
							},
						},
					},
					Quotas: config.Quotas{
						ServiceInstanceLimit: &dedicatedPlanQuota,
					},
					Properties: serviceadapter.Properties{
						"type": "dedicated-plan-property",
					},
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:               "instance-group-name",
							VMType:             dedicatedPlanVMType,
							VMExtensions:       dedicatedPlanVMExtensions,
							PersistentDiskType: dedicatedPlanDisk,
							Instances:          dedicatedPlanInstances,
							Networks:           dedicatedPlanNetworks,
							AZs:                dedicatedPlanAZs,
						},
						{
							Name:               "instance-group-errand",
							Lifecycle:          "errand",
							VMType:             dedicatedPlanVMType,
							PersistentDiskType: dedicatedPlanDisk,
							Instances:          dedicatedPlanInstances,
							Networks:           dedicatedPlanNetworks,
							AZs:                dedicatedPlanAZs,
						},
					},
				},
				{
					Name:        highMemoryPlanName,
					ID:          highMemoryPlanID,
					Description: highMemoryPlanDescription,
					Metadata: config.PlanMetadata{
						DisplayName: highMemoryPlanDisplayName,
						Bullets:     highMemoryPlanBullets,
					},
					Properties: serviceadapter.Properties{
						"type":            "high-memory-plan-property",
						"global_property": "overrides_global_value",
					},
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:         "instance-group-name",
							VMType:       highMemoryPlanVMType,
							VMExtensions: highMemoryPlanVMExtensions,
							Instances:    highMemoryPlanInstances,
							Networks:     highMemoryPlanNetworks,
							AZs:          highMemoryPlanAZs,
						},
					},
				},
			},
		},
	}
}

func killBrokerAndCheckForOpenConnections(session *gexec.Session, url string) {
	output, err := exec.Command("lsof", "-p", strconv.Itoa(session.Command.Process.Pid)).Output()
	Expect(err).NotTo(HaveOccurred(), string(output))
	Expect(string(output)).NotTo(ContainSubstring("can't identify protocol"))
	Expect(string(output)).NotTo(ContainSubstring(url))
	Eventually(session.Terminate()).Should(gexec.Exit())
}

func deploymentName(instanceID string) string {
	return "service-instance_" + instanceID
}

func rawManifestWithDeploymentName(instanceID string) string {
	return "name: " + deploymentName(instanceID)
}

func rawManifestInvalidReleaseVersion(instanceID string) string {
	return fmt.Sprintf(`name: %s
releases:
  - name: something
    version: latest
`, deploymentName(instanceID))
}

func rawManifestInvalidStemcellVersion(instanceID string) string {
	return fmt.Sprintf(`name: %s
stemcells:
  - name: something
    version: latest
`, deploymentName(instanceID))
}

func rawManifestFromBoshManifest(manifest bosh.BoshManifest) string {
	return string(toYaml(manifest))
}

func listCFServiceOfferingsResponse(serviceOfferingID, ccServiceOfferingGUID string) string {
	return `{
		"next_url": null,
		"resources": [
			{
				"entity": {
					"unique_id": "` + serviceOfferingID + `",
					"service_plans_url": "/v2/services/` + ccServiceOfferingGUID + `/service_plans"
				}
			}
		]
	}`
}

func listCFServiceInstanceCountForPlanResponse(count int) string {
	return `{
	 "total_results": ` + strconv.Itoa(count) + `
}`
}

func getServiceInstanceResponse(ccServicePlanGUID, lastOperationState string) string {
	return `{
	   "entity": {
	      "service_plan_url": "/v2/service_plans/` + ccServicePlanGUID + `",
				"last_operation": {
				  "state": "` + lastOperationState + `"
			  }
	   }
	}
`
}

func getServicePlanResponse(planID string) string {
	return `{
	   "entity": {
	      "unique_id": "` + planID + `"
	   }
	}
`
}

func booleanPointer(val bool) *bool {
	return &val
}

func readJSONResponse(reader io.ReadCloser) map[string]string {
	response := make(map[string]string)
	defer reader.Close()
	Expect(json.NewDecoder(reader).Decode(&response)).To(Succeed())
	return response
}

func logRegexpStringWithRequestIDCapture(message string) string {
	return fmt.Sprintf(`\[on-demand-service-broker\] \[([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} %s`, message)
}

func logRegexpString(requestID, message string) string {
	return fmt.Sprintf(`\[on-demand-service-broker\] \[%s\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} %s`, requestID, message)
}

func firstMatchInOutput(session *gexec.Session, regexpString string) string {
	logs := string(session.Buffer().Contents())
	return regexp.MustCompile(regexpString).FindStringSubmatch(logs)[1]
}

func toYaml(obj interface{}) []byte {
	data, err := yaml.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return data
}
