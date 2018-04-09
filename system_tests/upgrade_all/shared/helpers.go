package shared

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

type TestService struct {
	GUID    string
	Name    string
	AppName string
	AppURL  string
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

func ExtractPlanProperty(planName string, manifest *bosh.BoshManifest, brokerIGName, brokerJobName string) map[interface{}]interface{} {
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

func UpdatePlanProperties(brokerManifest *bosh.BoshManifest, config *Config, brokerIGName, brokerJobName string) {
	testPlan := ExtractPlanProperty(config.CurrentPlan, brokerManifest, brokerIGName, brokerJobName)
	testPlan["properties"] = map[interface{}]interface{}{"persistence": false}
}

func MigrateJobProperty(brokerManifest *bosh.BoshManifest, config *Config, brokerIGName, brokerJobName string) {
	testPlan := ExtractPlanProperty(config.CurrentPlan, brokerManifest, brokerIGName, brokerJobName)
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
}
