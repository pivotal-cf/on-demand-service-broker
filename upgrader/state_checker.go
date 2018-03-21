package upgrader

import (
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
)

type LastOperationChecker struct {
	brokerServices BrokerServices
}

func NewStateChecker(brokerServices BrokerServices) *LastOperationChecker {
	return &LastOperationChecker{
		brokerServices: brokerServices,
	}
}

func (l *LastOperationChecker) Check(guid string, operationData broker.OperationData) (services.UpgradeOperation, error) {
	lastOperation, err := l.brokerServices.LastOperation(guid, operationData)
	if err != nil {
		return services.UpgradeOperation{}, fmt.Errorf("error getting last operation: %s", err)
	}

	upgradeOperation := services.UpgradeOperation{Data: operationData, Description: lastOperation.Description}

	switch lastOperation.State {
	case brokerapi.Failed:
		upgradeOperation.Type = services.UpgradeFailed
	case brokerapi.Succeeded:
		upgradeOperation.Type = services.UpgradeSucceeded
	case brokerapi.InProgress:
		upgradeOperation.Type = services.UpgradeAccepted
	default:
		return services.UpgradeOperation{}, fmt.Errorf("uknown state from last operation: %s", lastOperation.State)
	}

	return upgradeOperation, nil
}
