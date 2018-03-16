// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_instances_errand_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"os"

	"log"

	"time"

	"regexp"

	"path"

	"github.com/craigfurman/herottp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	cfClient "github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

type testService struct {
	GUID    string
	Name    string
	AppName string
	AppURL  string
}

var serviceInstances []*testService
var canaryServiceInstances []*testService

var dataPersistenceEnabled bool

const brokerJobName = "broker"
const brokerIGName = "broker"

var _ = Describe("upgrade-all-service-instances errand", func() {
	var (
		filterParams map[string]string
		spaceName    string
	)

	BeforeEach(func() {
		spaceName = ""
		currentPlan = selectPlanName()
		dataPersistenceEnabled = checkDataPersistence()
		serviceInstances = []*testService{}
		filterParams = map[string]string{}
		cfTargetSpace(cfSpace)
	})

	AfterEach(func() {
		cfTargetSpace(cfSpace)
		deleteServiceInstances(serviceInstances)
		if spaceName != "" {
			cfTargetSpace(spaceName)
			deleteServiceInstances(canaryServiceInstances)
			cfDeleteSpace(spaceName)
		}
		boshClient.DeployODB(*originalBrokerManifest)
	})

	It("exits 1 when the upgrader fails", func() {
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
		serviceInstances = createServiceInstances()
		instances := []*testService{}
		upgradeInstanceProperties := findUpgradeAllServiceInstancesProperties(brokerManifest)

		if upgradeInstanceProperties["service_instances_api"] != nil && upgradeInstanceProperties["canary_selection_params"] != nil {
			filterParams = map[string]string{}
			for k, v := range upgradeInstanceProperties["canary_selection_params"].(map[interface{}]interface{}) {
				filterParams[k.(string)] = v.(string)
			}
			instances = serviceInstances[1:2]
		}

		By("causing an upgrade error")
		updateServiceInstancesAPI(brokerManifest, instances, filterParams)
		testPlan := extractPlanProperty(currentPlan, brokerManifest)

		redisServer := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})
		redisServer["vm_type"] = "doesntexist"

		By("deploying the broken broker manifest")
		boshClient.DeployODB(*brokerManifest)

		boshOutput := boshClient.RunErrandWithoutCheckingSuccess(brokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.ExitCode).To(Equal(1))
		Expect(boshOutput.StdOut).To(ContainSubstring("Upgrade failed"))
	})

	It("when there are no service instances provisioned, upgrade-all-service-instances runs successfully", func() {
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
		updateServiceInstancesAPI(brokerManifest, []*testService{}, map[string]string{})
		updatePlanProperties(brokerManifest)
		migrateJobProperty(brokerManifest)

		By("deploying the modified broker manifest")
		boshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := boshClient.RunErrand(brokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.ExitCode).To(Equal(0))
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))
	})

	It("when canaries from an org and space are required, they upgrade before the rest", func() {
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
		upgradeInstanceProperties := findUpgradeAllServiceInstancesProperties(brokerManifest)

		if upgradeInstanceProperties["canary_selection_params"] != nil {
			serviceInstances = createServiceInstances()
			filterParams := map[string]string{}

			var nonCanaryInstances []*testService

			for k, v := range upgradeInstanceProperties["canary_selection_params"].(map[interface{}]interface{}) {
				filterParams[k.(string)] = v.(string)
			}

			if upgradeInstanceProperties["service_instances_api"] != nil {
				canaryServiceInstances = serviceInstances[1:2]
				nonCanaryInstances = append(serviceInstances[:1], serviceInstances[2])
				updateServiceInstancesAPI(brokerManifest, canaryServiceInstances, filterParams)
			} else {
				spaceName = filterParams["cf_space"]
				cfCreateSpace(spaceName)
				cfTargetSpace(spaceName)

				canaryServiceInstances = createServiceInstances()
				nonCanaryInstances = serviceInstances
			}

			By("logging stdout to the errand output")
			boshOutput := boshClient.RunErrand(brokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")

			logMatcher := "(?s)STARTING CANARY UPGRADES(.*)FINISHED CANARY UPGRADES(.*)FINISHED UPGRADES"
			re := regexp.MustCompile(logMatcher)
			matches := re.FindStringSubmatch(boshOutput.StdOut)
			for _, instance := range canaryServiceInstances {
				Expect(matches[1]).To(ContainSubstring(instance.GUID), fmt.Sprintf("Canary instances %v not present in canary instances upgraded", canaryServiceInstances))
				Expect(matches[2]).NotTo(ContainSubstring(instance.GUID), fmt.Sprintf("Canary instances %v present in non-canary instances upgraded", canaryServiceInstances))
			}
			for _, instance := range nonCanaryInstances {
				Expect(matches[1]).NotTo(ContainSubstring(instance.GUID), fmt.Sprintf("Non-canary instances %v present in canary instances upgraded", nonCanaryInstances))
				Expect(matches[2]).To(ContainSubstring(instance.GUID), fmt.Sprintf("Non-canary instances %v not present in non-canary instances upgraded", nonCanaryInstances))
			}
		}
	})

	It("when there are multiple service instances provisioned, upgrade-all-service-instances runs successfully", func() {
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
		serviceInstances = createServiceInstances()
		instances := []*testService{}
		upgradeInstanceProperties := findUpgradeAllServiceInstancesProperties(brokerManifest)

		if upgradeInstanceProperties["canary_selection_params"] != nil {
			if upgradeInstanceProperties["service_instances_api"] != nil {
				filterParams = map[string]string{}
				for k, v := range upgradeInstanceProperties["canary_selection_params"].(map[interface{}]interface{}) {
					filterParams[k.(string)] = v.(string)
				}
				instances = serviceInstances[1:2]
			} else {
				Skip("omitting CF filtered canaries until refactor of system tests")
			}
		}

		updateServiceInstancesAPI(brokerManifest, instances, filterParams)
		updatePlanProperties(brokerManifest)
		migrateJobProperty(brokerManifest)

		By("deploying the modified broker manifest")
		boshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := boshClient.RunErrand(brokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))

		for _, service := range serviceInstances {
			deploymentName := getServiceDeploymentName(service.Name)
			manifest := boshClient.GetManifest(deploymentName)

			if dataPersistenceEnabled {
				By("ensuring data still exists", func() {
					Expect(cf.GetFromTestApp(service.AppURL, "foo")).To(Equal("bar"))
				})
			}

			By(fmt.Sprintf("upgrading instance '%s'", service.Name))
			instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(manifest, "redis")
			Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))

			expectedBoshTasksOrder := []string{"create deployment", "run errand", "create deployment", "run errand", "create deployment", "run errand"}
			if parallelUpgradesEnabled {
				// with one canary, we expect one upgrade to complete before the remaining two start simultaneously
				expectedBoshTasksOrder = []string{"create deployment", "run errand", "create deployment", "create deployment", "run errand", "run errand"}
			}

			if boshSupportsLifecycleErrands {
				By(fmt.Sprintf("running the post-deploy errand for instance '%s'", service.Name))
				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(service.Name))
				Expect(boshTasks).To(HaveLen(4))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring(expectedBoshTasksOrder[3]))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring(expectedBoshTasksOrder[2]))

				Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[2].Description).To(ContainSubstring(expectedBoshTasksOrder[1]))

				Expect(boshTasks[3].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[3].Description).To(ContainSubstring(expectedBoshTasksOrder[0]))
			}
		}
	})
})

func updatePlanProperties(brokerManifest *bosh.BoshManifest) {
	testPlan := extractPlanProperty(currentPlan, brokerManifest)
	testPlan["properties"] = map[interface{}]interface{}{"persistence": false}
}

func migrateJobProperty(brokerManifest *bosh.BoshManifest) {
	testPlan := extractPlanProperty(currentPlan, brokerManifest)
	brokerJobs := bosh_helpers.FindInstanceGroupJobs(brokerManifest, brokerIGName)
	serviceAdapterJob := extractServiceAdapterJob(brokerJobs)
	Expect(serviceAdapterJob).ToNot(BeNil(), "Couldn't find service adapter job in existing manifest")

	newRedisServerName := "redis"
	serviceAdapterJob.Properties["redis_instance_group_name"] = newRedisServerName

	testPlanInstanceGroup := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})

	oldRedisServerName := testPlanInstanceGroup["name"]

	testPlanInstanceGroup["name"] = newRedisServerName
	testPlanInstanceGroup["migrated_from"] = []map[interface{}]interface{}{
		{"name": oldRedisServerName},
	}
}

func updateServiceInstancesAPI(brokerManifest *bosh.BoshManifest, filteredServices []*testService, filterParams map[string]string) {
	upgradeInstanceProperties := findUpgradeAllServiceInstancesProperties(brokerManifest)
	if upgradeInstanceProperties["service_instances_api"] != nil {
		authHeaderBuilder, err := authorizationheader.NewUserTokenAuthHeaderBuilder(
			os.Getenv("CF_UAA_URL"),
			"cf",
			"",
			os.Getenv("CF_USERNAME"),
			os.Getenv("CF_PASSWORD"),
			true,
			[]byte(""),
		)
		Expect(err).NotTo(HaveOccurred())

		cfCli, err := cfClient.New(
			os.Getenv("CF_URL"),
			authHeaderBuilder,
			[]byte(""),
			true,
		)
		Expect(err).NotTo(HaveOccurred())

		logger := log.New(GinkgoWriter, "", log.LstdFlags)

		instances, err := cfCli.GetInstancesOfServiceOffering(serviceGUID, logger)
		Expect(err).NotTo(HaveOccurred())

		instancesJson, err := json.Marshal(instances)
		Expect(err).NotTo(HaveOccurred())

		var filteredInstances []service.Instance
		for _, instance := range instances {
			for _, filteredInstance := range filteredServices {
				if instance.GUID == filteredInstance.GUID {
					filteredInstances = append(filteredInstances, instance)
				}
			}
		}
		filteredInstancesJson, err := json.Marshal(filteredInstances)
		Expect(err).NotTo(HaveOccurred())

		serviceInstanceAPIConfig, ok := upgradeInstanceProperties["service_instances_api"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		url, ok := serviceInstanceAPIConfig["url"].(string)
		Expect(ok).To(BeTrue())
		authentication, ok := serviceInstanceAPIConfig["authentication"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		basic, ok := authentication["basic"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		username, ok := basic["username"].(string)
		Expect(ok).To(BeTrue())
		password, ok := basic["password"].(string)
		Expect(ok).To(BeTrue())

		Expect(url).NotTo(Equal(""), "url")
		Expect(username).NotTo(Equal(""), "username")
		Expect(password).NotTo(Equal(""), "password")

		url = strings.Replace(url, "https", "http", 1)

		httpClient := herottp.New(herottp.Config{
			Timeout: 30 * time.Second,
		})

		basicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
			username,
			password,
		)

		request, err := http.NewRequest(
			http.MethodPost,
			url,
			bytes.NewReader(instancesJson),
		)
		Expect(err).NotTo(HaveOccurred())

		err = basicAuthHeaderBuilder.AddAuthHeader(request, logger)
		Expect(err).NotTo(HaveOccurred())

		resp, err := httpClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		if len(filterParams) > 0 {
			filteredInstancesRequest, err := http.NewRequest(
				http.MethodPost,
				url,
				bytes.NewReader(filteredInstancesJson),
			)
			Expect(err).NotTo(HaveOccurred())
			q := filteredInstancesRequest.URL.Query()
			for k, v := range filterParams {
				q.Set(k, v)
			}
			filteredInstancesRequest.URL.RawQuery = q.Encode()

			err = basicAuthHeaderBuilder.AddAuthHeader(filteredInstancesRequest, logger)
			Expect(err).NotTo(HaveOccurred())

			filteredInstancesResponse, err := httpClient.Do(filteredInstancesRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(filteredInstancesResponse.StatusCode).To(Equal(http.StatusOK))
		}
	}
}

func findUpgradeAllServiceInstancesProperties(brokerManifest *bosh.BoshManifest) map[string]interface{} {
	return bosh_helpers.FindJobProperties(brokerManifest, "broker", "upgrade-all-service-instances")
}

func createServiceInstances() []*testService {
	var wg sync.WaitGroup

	newInstances := []*testService{
		{Name: uuid.New(), AppName: uuid.New()},
		{Name: uuid.New(), AppName: uuid.New()},
		{Name: uuid.New(), AppName: uuid.New()},
	}

	wg.Add(len(newInstances))

	for _, service := range newInstances {
		go func(ts *testService) {
			defer GinkgoRecover()
			defer wg.Done()

			By(fmt.Sprintf("Creating service instance: %s", ts.Name))
			createServiceSession := cf.Cf("create-service", serviceOffering, currentPlan, ts.Name)
			Eventually(createServiceSession, cf.CfTimeout).Should(
				gexec.Exit(0),
			)

			By(fmt.Sprintf("Polling for successful creation of service instance: %s", ts.Name))
			cf.AwaitServiceCreation(ts.Name)

			ts.GUID = cf.GetServiceInstanceGUID(ts.Name)

			if dataPersistenceEnabled {
				By("pushing an app and binding to it")
				ts.AppURL = cf.PushAndBindApp(
					ts.AppName,
					ts.Name,
					path.Join(ciRootPath, exampleAppDirName),
				)

				By("adding data to the service instance")
				cf.PutToTestApp(ts.AppURL, "foo", "bar")
			}
		}(service)
	}

	wg.Wait()
	return newInstances
}

func deleteServiceInstances(instancesToDelete []*testService) {
	var wg sync.WaitGroup

	for _, service := range instancesToDelete {
		wg.Add(1)
		go func(ts *testService) {
			defer GinkgoRecover()
			defer wg.Done()
			if dataPersistenceEnabled {
				By("unbinding the corresponding app")
				unbindServiceSession := cf.Cf("unbind-service", ts.AppName, ts.Name)
				Eventually(unbindServiceSession, cf.CfTimeout).Should(
					gexec.Exit(0),
				)

				By("deleting the corresponding app")
				deleteSession := cf.Cf("delete", ts.AppName, "-f", "-r")
				Eventually(deleteSession, cf.CfTimeout).Should(gexec.Exit(0))
			}

			By("deleting the service instance")
			deleteServiceSession := cf.Cf("delete-service", ts.Name, "-f")
			Eventually(deleteServiceSession, cf.CfTimeout).Should(
				gexec.Exit(0),
			)

			By("ensuring the service instance is deleted")
			cf.AwaitServiceDeletion(ts.Name)
		}(service)
	}

	wg.Wait()
}

func extractPlanProperty(planName string, manifest *bosh.BoshManifest) map[interface{}]interface{} {
	var brokerJob bosh.Job
	for _, ig := range manifest.InstanceGroups {
		if ig.Name == brokerIGName {
			for _, job := range ig.Jobs {
				if job.Name == brokerJobName {
					brokerJob = job
				}
			}
		}
	}

	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})

	for _, plan := range serviceCatalog["plans"].([]interface{}) {
		if plan.(map[interface{}]interface{})["name"] == planName {
			return plan.(map[interface{}]interface{})
		}
	}

	return nil
}

func extractServiceAdapterJob(jobs []bosh.Job) bosh.Job {
	for _, j := range jobs {
		if j.Name == "service-adapter" {
			return j
		}
	}

	return bosh.Job{}
}

func getServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf.CfTimeout).Should(gexec.Exit(0))
	re := regexp.MustCompile("(?m)^[[:alnum:]]{8}-[[:alnum:]-]*$")
	serviceGUID := re.FindString(string(getInstanceDetailsCmd.Out.Contents()))
	serviceInstanceID := strings.TrimSpace(serviceGUID)
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}

func selectPlanName() string {
	if boshSupportsLifecycleErrands {
		return "lifecycle-post-deploy-plan"
	} else {
		return "dedicated-vm"
	}
}

func checkDataPersistence() bool {
	return !boshSupportsLifecycleErrands
}
