package cf_helpers

import (
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/gomega"
)

func CreateServiceKey(serviceName, serviceKeyName string) {
	cfArgs := []string{"create-service-key", serviceName, serviceKeyName}

	Eventually(cf.Cf(cfArgs...), CfTimeout).Should(gexec.Exit(0))
}

func GetServiceKey(serviceName, serviceKeyName string) string {
	serviceKey := cf.Cf("service-key", serviceName, serviceKeyName)
	Eventually(serviceKey, CfTimeout).Should(gexec.Exit(0))
	serviceKeyContent := string(serviceKey.Buffer().Contents())

	return serviceKeyContent
}
