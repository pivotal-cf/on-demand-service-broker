package with_maintenance_info_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"

	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
)

func TestMaintenanceInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MaintenanceInfo Suite")
}

var (
	brokerInfo bosh.BrokerInfo
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	brokerInfo = bosh.DeployBroker(
		"-catalog-"+uniqueID,
		bosh.BrokerDeploymentOptions{},
		service_helpers.Redis,
		[]string{"update_service_catalog.yml"})
})

var _ = AfterSuite(func() {
	if os.Getenv("KEEP_ALIVE") != "true" {
		bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
	}
})
