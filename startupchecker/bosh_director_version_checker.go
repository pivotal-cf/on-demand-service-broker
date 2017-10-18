package startupchecker

import (
	"log"

	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type BOSHDirectorVersionChecker struct {
	boshInfo        *boshdirector.Info
	serviceOffering config.ServiceOffering
	logger          *log.Logger
}

func NewBOSHDirectorVersionChecker(
	boshInfo *boshdirector.Info,
	serviceOffering config.ServiceOffering,
	logger *log.Logger,
) *BOSHDirectorVersionChecker {
	return &BOSHDirectorVersionChecker{
		boshInfo:        boshInfo,
		serviceOffering: serviceOffering,
		logger:          logger,
	}
}

func (c *BOSHDirectorVersionChecker) Check() error {
	errPrefix := "BOSH Director error: "
	directorVersion, err := c.boshInfo.GetDirectorVersion(c.logger)

	if err != nil {
		return fmt.Errorf("%s%s. ODB requires BOSH v257+.", errPrefix, err)
	}
	if !directorVersion.SupportsODB() {
		return fmt.Errorf("%sAPI version is insufficient, ODB requires BOSH v257+.", errPrefix)
	}
	if c.serviceOffering.HasLifecycleErrands() && !directorVersion.SupportsLifecycleErrands() {
		return fmt.Errorf("%sAPI version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v%d+.", errPrefix, boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands)
	}

	return nil
}
