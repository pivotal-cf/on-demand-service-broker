// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/craigfurman/herottp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	cfClient "github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

type TestService struct {
	GUID    string
	Name    string
	AppName string
	AppURL  string
}

const (
	brokerIGName  = "broker"
	brokerJobName = "broker"
)

func CfCreateSpace(spaceName string) {
	Eventually(cf.Cf("create-space", spaceName)).Should(gexec.Exit(0))
}
func CfDeleteSpace(spaceName string) {
	Eventually(cf.Cf("delete-space", spaceName, "-f")).Should(gexec.Exit(0))
}

func CfTargetSpace(spaceName string) {
	Eventually(cf.Cf("target", "-s", spaceName)).Should(gexec.Exit(0))
}

func CreateServiceInstances(config *Config, dataPersistenceEnabled bool) []*TestService {
	var wg sync.WaitGroup

	newInstances := []*TestService{
		{Name: uuid.New(), AppName: uuid.New()},
		{Name: uuid.New(), AppName: uuid.New()},
		{Name: uuid.New(), AppName: uuid.New()},
	}

	wg.Add(len(newInstances))

	for _, service := range newInstances {
		go func(ts *TestService) {
			defer GinkgoRecover()
			defer wg.Done()

			By(fmt.Sprintf("Creating service instance: %s", ts.Name))
			createServiceSession := cf.Cf("create-service", config.ServiceOffering, config.CurrentPlan, ts.Name)
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
					path.Join(config.CiRootPath, config.ExampleAppDirName),
				)

				By("adding data to the service instance")
				cf.PutToTestApp(ts.AppURL, "foo", "bar")
			}
		}(service)
	}

	wg.Wait()
	return newInstances
}

type SIAPIConfig struct {
	URL      string
	Username string
	Password string
}

func UpdateServiceInstancesAPI(servicesApiConfig SIAPIConfig, filteredServices []*TestService, filterParams map[string]string, config *Config) {
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

	cfCli, err := cfClient.New(os.Getenv("CF_URL"), authHeaderBuilder, []byte(""), true)
	Expect(err).NotTo(HaveOccurred())

	logger := log.New(GinkgoWriter, "", log.LstdFlags)

	instances, err := cfCli.GetInstancesOfServiceOffering(config.ServiceGUID, logger)
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

	url := strings.Replace(servicesApiConfig.URL, "https", "http", 1)

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
	})

	basicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		servicesApiConfig.Username,
		servicesApiConfig.Password,
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

func DeleteServiceInstances(instancesToDelete []*TestService, dataPersistenceEnabled bool) {
	var wg sync.WaitGroup

	for _, service := range instancesToDelete {
		wg.Add(1)
		go func(ts *TestService) {
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

func ExtractPlanProperty(planName string, manifest *bosh.BoshManifest) map[interface{}]interface{} {
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

func ExtractServiceAdapterJob(jobs []bosh.Job) bosh.Job {
	for _, j := range jobs {
		if j.Name == "service-adapter" {
			return j
		}
	}

	return bosh.Job{}
}

func GetServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf.CfTimeout).Should(gexec.Exit(0))
	re := regexp.MustCompile("(?m)^[[:alnum:]]{8}-[[:alnum:]-]*$")
	serviceGUID := re.FindString(string(getInstanceDetailsCmd.Out.Contents()))
	serviceInstanceID := strings.TrimSpace(serviceGUID)
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}

func UpdatePlanProperties(brokerManifest *bosh.BoshManifest, config *Config) {
	testPlan := ExtractPlanProperty(config.CurrentPlan, brokerManifest)
	testPlan["properties"] = map[interface{}]interface{}{"persistence": false}
}

func ChangeInstanceGroupName(brokerManifest *bosh.BoshManifest, config *Config) {
	testPlan := ExtractPlanProperty(config.CurrentPlan, brokerManifest)
	brokerJobs := bosh_helpers.FindInstanceGroupJobs(brokerManifest, brokerIGName)
	serviceAdapterJob := ExtractServiceAdapterJob(brokerJobs)
	Expect(serviceAdapterJob).ToNot(BeNil(), "Couldn't find service adapter job in existing manifest")

	newRedisServerName := "redis"
	serviceAdapterJob.Properties["redis_instance_group_name"] = newRedisServerName

	testPlanInstanceGroup := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})

	oldRedisServerName := testPlanInstanceGroup["name"]

	testPlanInstanceGroup["name"] = newRedisServerName
	testPlanInstanceGroup["migrated_from"] = []map[interface{}]interface{}{
		{"name": oldRedisServerName},
	}

	dnsDetails, found := testPlan["binding_with_dns"]
	if !found {
		return
	}

	for _, detail := range dnsDetails.([]interface{}) {
		detailMap := detail.(map[interface{}]interface{})
		if detailMap["instance_group"] == oldRedisServerName {
			detailMap["instance_group"] = newRedisServerName
		}
	}
}

func FindUpgradeAllServiceInstancesProperties(brokerManifest *bosh.BoshManifest) map[string]interface{} {
	return bosh_helpers.FindJobProperties(brokerManifest, brokerJobName, "upgrade-all-service-instances")
}
func EnvMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
