package decider

import (
	"encoding/json"
	"fmt"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"log"
)

type Decider struct{}

func (d Decider) Decide(catalog []domain.Service, details domain.UpdateDetails, logger *log.Logger) (bool, error) {
	planMaintenanceInfo, err := getMaintenanceInfoForPlan(details.PlanID, catalog)
	if err != nil {
		return false, err
	}

	if maintenanceInfoConflict(details.MaintenanceInfo, planMaintenanceInfo) {
		return false, apiresponses.ErrMaintenanceInfoConflict
	}

	if planNotChanged(details) && requestParamsEmpty(details) {
		return true, nil
	}

	return false, nil
}

func planNotChanged(details domain.UpdateDetails) bool {
	return details.PlanID == details.PreviousValues.PlanID
}

func requestParamsEmpty(details domain.UpdateDetails) bool {
	var params map[string]interface{}
	if err := json.Unmarshal(details.RawParameters, &params); err != nil {
		return false
	}
	return len(params) == 0
}

func getMaintenanceInfoForPlan(id string, serviceCatalog []domain.Service) (*domain.MaintenanceInfo, error) {
	for _, plan := range serviceCatalog[0].Plans {
		if plan.ID == id {
			return plan.MaintenanceInfo, nil
		}
	}

	return nil, fmt.Errorf("plan %s does not exist", id)
}

func maintenanceInfoConflict(a, b *domain.MaintenanceInfo) bool {
	if a != nil && b != nil {
		return !a.Equals(*b)
	}

	if a == nil && b == nil {
		return false
	}

	return true
}
