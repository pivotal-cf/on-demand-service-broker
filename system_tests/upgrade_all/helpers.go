package upgrade_all

import (
	"sync"

	"github.com/onsi/ginkgo"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

type AppDetails struct {
	UUID                  string
	AppURL                string
	AppName               string
	ServiceName           string
	ServiceGUID           string
	ServiceDeploymentName string
}

func PerformInParallel(createFunction func(planName string), count int, planNames *PlanNamesForParallelCreate) {
	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(i int) {
			defer ginkgo.GinkgoRecover()
			defer wg.Done()
			createFunction(planNames.Get(i))
		}(i)
	}

	wg.Wait()
}

func CreateServiceAndApp(serviceOffering, planName string) AppDetails {
	uuid := uuid.New()[:8]
	serviceName := "service-" + uuid
	appName := "app-" + uuid
	cf_helpers.CreateService(serviceOffering, planName, serviceName, "")
	serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)

	appPath := cf_helpers.GetAppPath(service_helpers.Redis)
	appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
	cf_helpers.PutToTestApp(appURL, "uuid", uuid)

	return AppDetails{
		UUID:                  uuid,
		AppURL:                appURL,
		AppName:               appName,
		ServiceName:           serviceName,
		ServiceGUID:           serviceGUID,
		ServiceDeploymentName: "service-instance_" + serviceGUID,
	}
}

type PlanNamesForParallelCreate struct {
	sync.RWMutex
	Items []PlanName
}

type PlanName struct {
	Index int
	Value string
}

func (cs *PlanNamesForParallelCreate) Get(i int) string {
	cs.Lock()
	defer cs.Unlock()

	item := cs.Items[i]

	return item.Value
}
