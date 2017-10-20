package startupchecker_test

import (
	"fmt"
	"strconv"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("BOSH Director Version Checker", func() {
	var (
		serviceCatalog       config.ServiceOffering
		postDeployErrandPlan config.Plan
	)

	BeforeEach(func() {
		serviceCatalog = config.ServiceOffering{}
		postDeployErrandPlan = config.Plan{
			ID: "post-deploy",
			LifecycleErrands: &config.LifecycleErrands{
				PostDeploy: config.Errand{
					Name: "health-check",
				},
			},
			InstanceGroups: []serviceadapter.InstanceGroup{},
		}
	})

	It("returns no error when the BOSH director version supports ODB and lifecycle errands are not configured", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the BOSH director version does not support ODB", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB-1,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError("BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))
	})

	It("returns no error when BOSH director supports lifecycle errands", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
			boshdirector.VersionType("semver"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the BOSH director version does not support lifecycle errands and lifecycle errands are configured", func() {
		serviceCatalog.Plans = []config.Plan{postDeployErrandPlan}
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError("BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))
	})

	It("returns no error when the BOSH director version does not support lifecycle errands and lifecycle errands are not configured", func() {
		emptyLifecycleErrandsPlan := config.Plan{
			ID: "empty-lifecycle-errands-plan-id",
			LifecycleErrands: &config.LifecycleErrands{
				PostDeploy: config.Errand{},
				PreDelete:  "",
			},
		}

		serviceCatalog.Plans = config.Plans{
			emptyLifecycleErrandsPlan,
		}
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the BOSH director version in unrecognised", func() {
		boshInfo := boshdirector.Info{Version: "0000 (00000000)"}
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError(`BOSH Director error: unrecognised BOSH Director version: "0000 (00000000)". ODB requires BOSH v257+.`))
	})
})

func createBOSHInfoWithMajorVersion(majorVersion int, versionType boshdirector.VersionType) boshdirector.Info {
	var version string
	if versionType == "semver" {
		version = fmt.Sprintf("%s.0.0", strconv.Itoa(majorVersion))
	} else if versionType == "stemcell" {
		version = fmt.Sprintf("1.%s.0.0", strconv.Itoa(majorVersion))
	}
	return boshdirector.Info{Version: version}
}

func createBOSHDirectorVersionChecker(boshInfo boshdirector.Info, catalog config.ServiceOffering) *BOSHDirectorVersionChecker {
	return NewBOSHDirectorVersionChecker(
		broker.MinimumMajorStemcellDirectorVersionForODB,
		broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
		boshInfo,
		catalog,
	)
}
