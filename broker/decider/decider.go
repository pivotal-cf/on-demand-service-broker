package decider

import (
	"fmt"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"log"
)

type Decider struct{}

func (d Decider) Decide(catalog []domain.Service, details domain.UpdateDetails, logger *log.Logger) error {
	for _, plan := range catalog[0].Plans {
		if plan.ID == details.PlanID {

			if maintenanceInfoEqual(details.MaintenanceInfo, plan.MaintenanceInfo) {
				return nil
			}

			return apiresponses.ErrMaintenanceInfoConflict
		}
	}

	return fmt.Errorf("plan %s does not exist", details.PlanID)
}

//	RequestPlanID            string
//	RequestMaintenanceInfo   *domain.MaintenanceInfo
//	RequestParametersPresent bool
//	PreviousPlanID           string
//	PreviousMaintenanceInfo  *domain.MaintenanceInfo
//	ServiceCatalog           []domain.Service
//
//	// details.PlanID : the desired plan (when changing plans)
//	// details.MaintenanceInfo: the MI for the desired plan
//	// details.PreviousValues.PlanID: the current plan id
//	// details.PreviousValues.MaintenanceInfo: the current plan MI
//	//
//	// details.RawParameters: arbitrary parameters, if present it's an update
//	//
//	// PlanMaintenanceInfo: the current MI for the desired plan (as in the broker catalog)
//	// PreviousMaintenanceInfo: the current MI for the current plan (as in the broker catalog)
//
//	/*
//		Updates:
//		- has RawParameters (invalid if MI also changes)
//		- change of plan (invalid if MI also changes)
//
//		- errors if previous.MI doesn't match current MI for the Previous.Plan
//
//		Upgrades:
//		- change of MI
//	*/
//}
//
//func (d Decider) Decide() (Type, error) {
//	checker := maintenanceinfo.Checker{}
//
//	// TODO: we need to pass a logger, otherwise this will sometimes *panic*
//	// TODO: should we inline this check?
//	if err := checker.Check(d.RequestPlanID, d.RequestMaintenanceInfo, d.ServiceCatalog, nil); err != nil {
//		return Failed, fmt.Errorf("decider validation failed: %w"
//
//	if d.RequestParametersPresent || d.RequestPlanID != d.PreviousPlanID {
//		previousPlanMaintenanceInfo, err := getMaintenanceInfoForPlan(d.PreviousPlanID, d.ServiceCatalog)
//		if err != nil {
//			return Failed, err
//		}
//		if !maintenanceInfoEqual(previousPlanMaintenanceInfo, d.PreviousMaintenanceInfo) {
//			return Failed, fmt.Errorf("service instance needs to be upgraded before updating")
//		}
//
//		return Update, nil
//	}
//
//	return Upgrade, nil
//}
//
//func getMaintenanceInfoForPlan(id string, serviceCatalog []domain.Service) (*domain.MaintenanceInfo, error) {
//	for _, plan := range serviceCatalog[0].Plans {
//		if plan.ID == id {
//			return plan.MaintenanceInfo, nil
//		}
//	}
//
//	return nil, fmt.Errorf("plan %s not found", id)
//}
//
func maintenanceInfoEqual(a, b *domain.MaintenanceInfo) bool {
	if a != nil && b != nil {
		return a.Equals(*b)
	}

	if a == nil && b == nil {
		return true
	}

	return false
}

//
//type Type int
//
//const (
//	Update Type = iota
//	Upgrade
//	Failed
//)
