package with_maintenance_info_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
)

func TestServiceCatalog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceCatalog Suite")
}

var (
	brokerInfo bosh.BrokerInfo
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	brokerInfo = bosh.DeployAndRegisterBroker("-catalog-"+uniqueID, "update_service_catalog.yml")
})

var _ = AfterSuite(func() {
	if os.Getenv("KEEP_ALIVE") != "true" {
		bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
	}
})
