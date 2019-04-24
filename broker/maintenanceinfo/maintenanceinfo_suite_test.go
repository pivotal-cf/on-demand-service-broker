package maintenanceinfo_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMaintenanceinfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Maintenanceinfo Suite")
}
