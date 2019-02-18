package runtimechecker_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/runtimechecker"
)

var _ = Describe("RecreateRuntimeChecker", func() {
	Describe("Check", func() {
		It("succeeds if the BOSH version is above the minimum requirement", func() {
			for _, version := range []string{"266.15.0", "267.9.0", "268.2.2", "268.4.0", "270.0.0"} {
				checker := runtimechecker.RecreateRuntimeChecker{BoshInfo: boshdirector.Info{Version: version}}
				Expect(checker.Check()).To(Succeed(), version)
			}
		})

		It("fails if the BOSH version is below the minimum requirement", func() {
			for _, version := range []string{"265.0.0", "266.14.9", "267.8.9", "268.2.1", "268.3.9"} {
				checker := runtimechecker.RecreateRuntimeChecker{BoshInfo: boshdirector.Info{Version: version}}
				Expect(checker.Check()).To(MatchError(errors.New(fmt.Sprintf("Insufficient BOSH director version: %q. The recreate-all errand requires a BOSH director version 268.4.0 or higher, or one of the following patch releases: 266.15.0+, 267.9.0+, 268.2.2+.", version))))
			}

		})
	})
})
