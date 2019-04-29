package broker

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"

	"github.com/pborman/uuid"
)

func (b *Broker) Recreate(ctx context.Context, instanceID string, details domain.UpdateDetails, logger *log.Logger) (OperationData, error) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	logger.Printf("recreating instance %s", instanceID)

	var boshContextID string

	if details.PlanID == "" {
		return OperationData{}, b.processError(errors.New("no plan ID provided in recreate request body"), logger)
	}

	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		logger.Printf("error: finding plan ID %s", details.PlanID)
		return OperationData{}, b.processError(fmt.Errorf("plan %s not found", details.PlanID), logger)
	}

	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
	}

	taskID, err := b.deployer.Recreate(deploymentName(instanceID), details.PlanID, boshContextID, logger)

	if err != nil {
		logger.Printf("error recreating instance %s: %s", instanceID, err)

		switch err := err.(type) {
		case serviceadapter.UnknownFailureError:
			return OperationData{}, b.processError(adapterToAPIError(ctx, err), logger)
		case TaskInProgressError:
			return OperationData{}, b.processError(NewOperationInProgressError(err), logger)
		default:
			return OperationData{}, b.processError(err, logger)
		}
	}

	return OperationData{
		BoshContextID: boshContextID,
		BoshTaskID:    taskID,
		OperationType: OperationTypeRecreate,
		Errands:       plan.PostDeployErrands(),
	}, nil
}
