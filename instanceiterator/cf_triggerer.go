package instanceiterator

import (
	"fmt"
	"log"

	"github.com/coreos/go-semver/semver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pkg/errors"
)

//go:generate counterfeiter -o fakes/fake_cf_client.go . CFClient
type CFClient interface {
	GetOSBAPIVersion(logger *log.Logger) *semver.Version
	UpgradeServiceInstance(serviceInstanceGUID string, maintenanceInfo cf.MaintenanceInfo, logger *log.Logger) (cf.LastOperation, error)
	GetServiceInstance(serviceInstanceGUID string, logger *log.Logger) (cf.ServiceInstanceResource, error)
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

func (t *CFTriggerer) TriggerOperation(instance service.Instance) (services.BOSHOperation, error) {
	servicePlan, err := t.cfClient.GetPlanByServiceInstanceGUID(instance.GUID, t.logger)
	if err != nil {
		return services.BOSHOperation{}, errors.Wrap(err, fmt.Sprintf("failed to trigger operation for instance %q", instance.GUID))
	}

	lastOperation, err := t.cfClient.UpgradeServiceInstance(instance.GUID, servicePlan.ServicePlanEntity.MaintenanceInfo, t.logger)
	if err != nil {
		return services.BOSHOperation{}, errors.Wrap(err, fmt.Sprintf("failed to trigger operation for instance %q", instance.GUID))
	}

	operationType := handleUpgradeResponse(lastOperation)

	return services.BOSHOperation{
		Type: operationType, // TODO: what other properties of BOSH operation are used in iterator?
	}, nil
}

func (t *CFTriggerer) Check(serviceInstanceGUID string, operationData broker.OperationData) (services.BOSHOperation, error) {
	serviceInstanceResource, err := t.cfClient.GetServiceInstance(serviceInstanceGUID, t.logger)
	if err != nil {
		return services.BOSHOperation{}, errors.Wrap(err, fmt.Sprintf("failed to check operation for instance %q", serviceInstanceGUID))
	}
	operationType := handleUpgradeResponse(serviceInstanceResource.Entity.LastOperation)

	return services.BOSHOperation{
		Type: operationType,
	}, nil
}

func handleUpgradeResponse(lastOperation cf.LastOperation) services.BOSHOperationType {
	var operationType services.BOSHOperationType
	switch lastOperation.State {
	case cf.OperationStateSucceeded:
		operationType = services.OperationSucceeded
	case cf.OperationStateInProgress:
		operationType = services.OperationAccepted // TODO Accepted or InProgress??
	case cf.OperationStateFailed:
		operationType = services.OperationFailed
	}
	return operationType
}
