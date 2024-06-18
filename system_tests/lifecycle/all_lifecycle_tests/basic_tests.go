package all_lifecycle_tests

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func BasicLifecycleTest(
	serviceType service_helpers.ServiceType,
	brokerInfo bosh_helpers.BrokerInfo,
	plan string,
	newPlanName string,
	arbitraryParams string,
) {
	var (
		serviceInstanceName string
		serviceKeyName      string
		appName             string
		appURL              string
	)

	By("creating a service", func() {
		serviceInstanceName = "service" + brokerInfo.TestSuffix
		cf_helpers.CreateService(brokerInfo.ServiceName, plan, serviceInstanceName, "")
	})

	By("creating a service key", func() {
		serviceKeyName = "serviceKey" + brokerInfo.TestSuffix
		cf_helpers.CreateServiceKey(serviceInstanceName, serviceKeyName)
		serviceKeyContents := cf_helpers.GetServiceKey(serviceInstanceName, serviceKeyName)
		cf_helpers.LooksLikeAServiceKey(serviceKeyContents)
	})

	By("binding an app", func() {
		appName = "example-app" + brokerInfo.TestSuffix
		appPath := cf_helpers.GetAppPath(serviceType)
		appURL = cf_helpers.PushAndBindApp(appName, serviceInstanceName, appPath)
	})

	By("testing the app can communicate with service", func() {
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating the plan for the service", func() {
		cf_helpers.UpdateServiceToPlan(serviceInstanceName, newPlanName)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("testing the app works after updating arbitrary parameters for the service", func() {
		cf_helpers.UpdateServiceWithArbitraryParams(serviceInstanceName, arbitraryParams)
		cf_helpers.ExerciseApp(serviceType, appURL)
	})

	By("unbinding the app", func() {
		cf_helpers.UnbindAndDeleteApp(appName, serviceInstanceName)
	})

	By("deleting a service key", func() {
		cf_helpers.DeleteServiceKey(serviceInstanceName, serviceKeyName)
	})

	By("deleting the service", func() {
		cf_helpers.DeleteService(serviceInstanceName)
	})
}
