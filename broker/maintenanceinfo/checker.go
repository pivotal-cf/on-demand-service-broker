package maintenanceinfo

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
)

type Checker struct{}

func (c Checker) Check(
	planID string,
	maintenanceInfo *domain.MaintenanceInfo,
	serviceCatalog []domain.Service,
	logger *log.Logger) error {

	planMaintenanceInfo, err := c.getMaintenanceInfoForPlan(planID, serviceCatalog)
	if err != nil {
		return err
	}

	if maintenanceInfo != nil {
		if planMaintenanceInfo == nil {
			return apiresponses.ErrMaintenanceInfoNilConflict
		}

		if !planMaintenanceInfo.Equals(*maintenanceInfo) {
			return apiresponses.ErrMaintenanceInfoConflict
		}
		return nil
	}

	if planMaintenanceInfo != nil {
		logger.Println("warning: maintenance info defined in broker service catalog, but not passed in request")
	}

	return nil
}

func (c Checker) getMaintenanceInfoForPlan(id string, serviceCatalog []domain.Service) (*domain.MaintenanceInfo, error) {
	for _, plan := range serviceCatalog[0].Plans {
		if plan.ID == id {
			return plan.MaintenanceInfo, nil
		}
	}

	return nil, fmt.Errorf("plan %s not found", id)
}
