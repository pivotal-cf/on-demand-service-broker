package startupchecker

import (
	"errors"
	"fmt"
	"log"

	"github.com/coreos/go-semver/semver"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type CFStartupChecker struct {
	cfClient         CloudFoundryClient
	minimumCFVersion string
	serviceOffering  config.ServiceOffering
	logger           *log.Logger
}

func NewCFChecker(cfClient CloudFoundryClient, minimumCFVersion string, serviceOffering config.ServiceOffering, logger *log.Logger) *CFStartupChecker {
	return &CFStartupChecker{
		cfClient:         cfClient,
		minimumCFVersion: minimumCFVersion,
		serviceOffering:  serviceOffering,
		logger:           logger,
	}
}

func (c *CFStartupChecker) Check() error {
	rawCFAPIVersion, err := c.cfClient.GetAPIVersion(c.logger)
	if err != nil {
		return errors.New("CF API error: " + err.Error() + ". ODB requires CF v238+.")
	}

	version, err := semver.NewVersion(rawCFAPIVersion)
	if err != nil {
		return fmt.Errorf("Cloud Foundry API version couldn't be parsed. Expected a semver, got: %s.", rawCFAPIVersion)
	}

	if version.LessThan(*semver.New(c.minimumCFVersion)) {
		return errors.New("CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+.")
	}

	instanceCountByPlanID, err := c.cfClient.CountInstancesOfServiceOffering(c.serviceOffering.ID, c.logger)
	if err != nil {
		return err
	}

	for plan, count := range instanceCountByPlanID {
		_, found := c.serviceOffering.Plans.FindByID(plan.ServicePlanEntity.UniqueID)

		if !found && count > 0 {
			return fmt.Errorf(
				"plan %s (%s) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances",
				plan.ServicePlanEntity.Name,
				plan.ServicePlanEntity.UniqueID,
			)
		}
	}

	return nil
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetAPIVersion(logger *log.Logger) (string, error)
	CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[cf.ServicePlan]int, err error)
}
