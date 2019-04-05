package upgrade_all

import (
	"sync"

	"github.com/onsi/ginkgo"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

type AppDetails struct {
	Uuid                  string
	AppURL                string
	AppName               string
	ServiceName           string
	ServiceGUID           string
	ServiceDeploymentName string
}

func PerformInParallel(f func(), count int) {
	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer ginkgo.GinkgoRecover()
			defer wg.Done()
			f()
		}()
	}

	wg.Wait()
}

func DeployService(serviceOffering, planName, appPath string) AppDetails {
	uuid := uuid.New()[:8]
	serviceName := "service-" + uuid
	appName := "app-" + uuid
	cf_helpers.CreateService(serviceOffering, planName, serviceName, "")
	serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)
	appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
	cf_helpers.PutToTestApp(appURL, "uuid", uuid)

	return AppDetails{
		Uuid:                  uuid,
		AppURL:                appURL,
		AppName:               appName,
		ServiceName:           serviceName,
		ServiceGUID:           serviceGUID,
		ServiceDeploymentName: "service-instance_" + serviceGUID,
	}
}
