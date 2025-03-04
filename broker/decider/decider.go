package decider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
)

var errInstanceMustBeUpgradedFirst = apiresponses.NewFailureResponseBuilder(
	errors.New("service instance needs to be upgraded before updating"),
	http.StatusUnprocessableEntity,
	"previous-maintenance-info-check",
).Build()

var warningMaintenanceInfoNilInTheRequest = errors.New(
	"maintenance info defined in broker service catalog, but not passed in request",
)

type Decider struct{}

type Operation int

const (
	Update Operation = iota
	Upgrade
	Failed
)

func (d Decider) CanProvision(catalog []domain.Service, planID string, maintenanceInfo *domain.MaintenanceInfo, logger *log.Logger) error {
	if err := validateMaintenanceInfo(catalog, planID, maintenanceInfo, logger); err != nil {
		if err != warningMaintenanceInfoNilInTheRequest {
			return err
		}
	}
	return nil
}

func (d Decider) DecideOperation(catalog []domain.Service, details domain.UpdateDetails, logger *log.Logger) (Operation, error) {
	if err := validateMaintenanceInfo(catalog, details.PlanID, details.MaintenanceInfo, logger); err != nil {
		if err == warningMaintenanceInfoNilInTheRequest {
			if err = validatePreviousMaintenanceInfo(details, catalog); err == nil {
				return Update, nil
			}
		}
		return Failed, err
	}

	if planNotChanged(details) && requestParamsEmpty(details) && requestMaintenanceInfoValuesDiffer(details) {
		return Upgrade, nil
	}

	if err := validatePreviousMaintenanceInfo(details, catalog); err != nil {
		return Failed, err
	}

	return Update, nil
}

func validatePreviousMaintenanceInfo(details domain.UpdateDetails, catalog []domain.Service) error {
	if details.PreviousValues.MaintenanceInfo != nil {
		if previousPlanMaintenanceInfo, err := getMaintenanceInfoForPlan(details.PreviousValues.PlanID, catalog); err == nil {
			if maintenanceInfoConflict(details.PreviousValues.MaintenanceInfo, previousPlanMaintenanceInfo) {
				return errInstanceMustBeUpgradedFirst
			}
		}
	}
	return nil
}

func validateMaintenanceInfo(catalog []domain.Service, planID string, maintenanceInfo *domain.MaintenanceInfo, logger *log.Logger) error {
	planMaintenanceInfo, err := getMaintenanceInfoForPlan(planID, catalog)
	if err != nil {
		return err
	}

	if maintenanceInfoConflict(maintenanceInfo, planMaintenanceInfo) {
		if maintenanceInfo == nil {
			logger.Printf("warning: %s\n", warningMaintenanceInfoNilInTheRequest)
			return warningMaintenanceInfoNilInTheRequest
		}

		if planMaintenanceInfo == nil {
			return apiresponses.ErrMaintenanceInfoNilConflict
		}

		return apiresponses.ErrMaintenanceInfoConflict
	}

	return nil
}

func requestMaintenanceInfoValuesDiffer(details domain.UpdateDetails) bool {
	if details.MaintenanceInfo == nil && details.PreviousValues.MaintenanceInfo != nil {
		return true
	}

	if details.MaintenanceInfo != nil && details.PreviousValues.MaintenanceInfo == nil {
		return true
	}

	if details.MaintenanceInfo == nil && details.PreviousValues.MaintenanceInfo == nil {
		return false
	}

	return !details.MaintenanceInfo.Equals(*details.PreviousValues.MaintenanceInfo)
}

func planNotChanged(details domain.UpdateDetails) bool {
	return details.PlanID == details.PreviousValues.PlanID
}

func requestParamsEmpty(details domain.UpdateDetails) bool {
	if len(details.RawParameters) == 0 {
		return true
	}

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
