package startupchecker

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type CFPlanConsistencyChecker struct {
	cfClient        ServiceInstanceCounter
	serviceOffering config.ServiceOffering
	logger          *log.Logger
}

func NewCFPlanConsistencyChecker(cfClient ServiceInstanceCounter, serviceOffering config.ServiceOffering, logger *log.Logger) *CFPlanConsistencyChecker {
	return &CFPlanConsistencyChecker{
		cfClient:        cfClient,
		serviceOffering: serviceOffering,
		logger:          logger,
	}
}

func (c *CFPlanConsistencyChecker) Check() error {
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

//go:generate counterfeiter -o fakes/fake_service_instance_counter.go . ServiceInstanceCounter
type ServiceInstanceCounter interface {
	CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[cf.ServicePlan]int, err error)
}
