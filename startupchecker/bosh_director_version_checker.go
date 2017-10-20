package startupchecker

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type BOSHDirectorVersionChecker struct {
	minimumMajorStemcellDirectorVersionForODB            int
	minimumMajorSemverDirectorVersionForLifecycleErrands int
	boshInfo                                             boshdirector.Info
	serviceOffering                                      config.ServiceOffering
}

func NewBOSHDirectorVersionChecker(
	minimumMajorStemcellDirectorVersionForODB int,
	minimumMajorSemverDirectorVersionForLifecycleErrands int,
	boshInfo boshdirector.Info,
	serviceOffering config.ServiceOffering,
) *BOSHDirectorVersionChecker {
	return &BOSHDirectorVersionChecker{
		minimumMajorStemcellDirectorVersionForODB:            minimumMajorStemcellDirectorVersionForODB,
		minimumMajorSemverDirectorVersionForLifecycleErrands: minimumMajorSemverDirectorVersionForLifecycleErrands,
		boshInfo:        boshInfo,
		serviceOffering: serviceOffering,
	}
}

func (c *BOSHDirectorVersionChecker) Check() error {
	errPrefix := "BOSH Director error: "
	directorVersion, err := c.boshInfo.GetDirectorVersion()

	if err != nil {
		return fmt.Errorf("%s%s. ODB requires BOSH v257+.", errPrefix, err)
	}
	if !c.directorVersionSufficientForODB(directorVersion) {
		return fmt.Errorf("%sAPI version is insufficient, ODB requires BOSH v257+.", errPrefix)
	}
	if c.serviceOffering.HasLifecycleErrands() && !c.directorVersionSufficientForLifecycleErrands(directorVersion) {
		return fmt.Errorf(
			"%sAPI version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v%d+.",
			errPrefix,
			c.minimumMajorSemverDirectorVersionForLifecycleErrands,
		)
	}

	return nil
}

func (c *BOSHDirectorVersionChecker) directorVersionSufficientForODB(directorVersion boshdirector.Version) bool {
	return directorVersion.VersionType == boshdirector.SemverDirectorVersionType ||
		directorVersion.MajorVersion >= c.minimumMajorStemcellDirectorVersionForODB
}

func (c *BOSHDirectorVersionChecker) directorVersionSufficientForLifecycleErrands(directorVersion boshdirector.Version) bool {
	return directorVersion.VersionType == boshdirector.SemverDirectorVersionType &&
		directorVersion.MajorVersion >= c.minimumMajorSemverDirectorVersionForLifecycleErrands
}
