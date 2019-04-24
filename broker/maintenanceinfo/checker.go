package maintenanceinfo

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/brokerapi"
)

type Checker struct{}

func (c Checker) Check(
	planID string,
	maintenanceInfo brokerapi.MaintenanceInfo,
	serviceCatalog []brokerapi.Service,
	logger *log.Logger) error {

	planMaintenanceInfo, err := c.getMaintenanceInfoForPlan(planID, serviceCatalog)
	if err != nil {
		return err
	}

	if !maintenanceInfo.NilOrEmpty() {
		if planMaintenanceInfo.NilOrEmpty() {
			return brokerapi.ErrMaintenanceInfoNilConflict
		}

		if !planMaintenanceInfo.Equals(maintenanceInfo) {
			return brokerapi.ErrMaintenanceInfoConflict
		}
		return nil
	}

	if !planMaintenanceInfo.NilOrEmpty() {
		logger.Println("warning: maintenance info defined in broker service catalog, but not passed in request")
	}

	return nil
}

func (c Checker) getMaintenanceInfoForPlan(id string, serviceCatalog []brokerapi.Service) (*brokerapi.MaintenanceInfo, error) {
	for _, plan := range serviceCatalog[0].Plans {
		if plan.ID == id {
			return plan.MaintenanceInfo, nil
		}
	}

	return nil, fmt.Errorf("plan %s not found", id)
}
