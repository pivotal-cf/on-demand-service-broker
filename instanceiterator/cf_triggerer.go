package instanceiterator

import (
	"log"

	"github.com/coreos/go-semver/semver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pkg/errors"
)

//go:generate counterfeiter -o fakes/fake_cf_client.go . CFClient
type CFClient interface {
	GetOSBAPIVersion(logger *log.Logger) *semver.Version
	GetServiceInstance(serviceInstanceGUID string, logger *log.Logger) (cf.ServiceInstanceResource, error)
	UpgradeServiceInstance(serviceInstanceGUID string, maintenanceInfo cf.MaintenanceInfo, logger *log.Logger) (cf.LastOperation, error)
	GetLastOperationForInstance(serviceInstanceGUID string, logger *log.Logger) (cf.LastOperation, error)
	GetPlanByServiceInstanceGUID(planUniqueID string, logger *log.Logger) (cf.ServicePlan, error)
}

type CFTriggerer struct {
	cfClient CFClient
	logger   *log.Logger
}

func NewCFTrigger(client CFClient, logger *log.Logger) *CFTriggerer {
	return &CFTriggerer{
		cfClient: client,
		logger:   logger,
	}
}

func (t *CFTriggerer) TriggerOperation(instance service.Instance) (TriggeredOperation, error) {
	servicePlan, err := t.cfClient.GetPlanByServiceInstanceGUID(instance.GUID, t.logger)
	if err != nil {
		return TriggeredOperation{}, errors.Wrapf(err, "failed to trigger operation for instance %q", instance.GUID)
	}

	instanceDetails, err := t.cfClient.GetServiceInstance(instance.GUID, t.logger)
	if err != nil {
		return TriggeredOperation{}, errors.Wrapf(err, "failed to get service instance %q", instance.GUID)
	}

	if instanceDetails.Entity.MaintenanceInfo.Version == servicePlan.ServicePlanEntity.MaintenanceInfo.Version {
		return TriggeredOperation{
			State: OperationSkipped,
		}, nil
	}

	lastOperation, err := t.cfClient.UpgradeServiceInstance(instance.GUID, servicePlan.ServicePlanEntity.MaintenanceInfo, t.logger)
	if err != nil {
		return TriggeredOperation{}, errors.Wrapf(err, "failed to trigger operation for instance %q", instance.GUID)
	}

	return translateUpgradeResponse(lastOperation), nil
}

func (t *CFTriggerer) Check(serviceInstanceGUID string, operationData broker.OperationData) (TriggeredOperation, error) {
	lastOperation, err := t.cfClient.GetLastOperationForInstance(serviceInstanceGUID, t.logger)
	if err != nil {
		return TriggeredOperation{}, errors.Wrapf(err, "failed to check operation for instance %q", serviceInstanceGUID)
	}

	return translateUpgradeResponse(lastOperation), nil
}

func translateUpgradeResponse(lastOperation cf.LastOperation) TriggeredOperation {
	var operationState OperationState
	switch lastOperation.State {
	case cf.OperationStateSucceeded:
		operationState = OperationSucceeded
	case cf.OperationStateInProgress:
		operationState = OperationAccepted
	case cf.OperationStateFailed:
		operationState = OperationFailed
	}
	return TriggeredOperation{
		State: operationState,
	}
}
