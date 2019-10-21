package updateparser

import (
	"fmt"

	"github.com/pivotal-cf/brokerapi/domain"
)

type UpdateParser struct {
	Details             domain.UpdateDetails
	PlanMaintenanceInfo *domain.MaintenanceInfo
}

func (u UpdateParser) IsUpgrade() (bool, error) {
	// Validate all MIs?

	if isSet(u.Details.MaintenanceInfo) {
		if !equal(u.Details.MaintenanceInfo, u.PlanMaintenanceInfo) {
			return false, fmt.Errorf("plan error")
		}
	}

	return u.Details.MaintenanceInfo != nil, nil
}

func isSet(maintenanceInfo *domain.MaintenanceInfo) bool {
	return maintenanceInfo != nil
}

func equal(a, b *domain.MaintenanceInfo) bool {
	if a != nil && b != nil {
		return a.Equals(*b)
	}
	return a == b
}
