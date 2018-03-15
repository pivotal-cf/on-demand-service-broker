package upgrader

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type instanceInfo struct {
	status        services.UpgradeOperationType
	initialPlan   string
	couldBeCanary bool
}

type upgradeState struct {
	guids           []string
	states          map[string]instanceInfo
	processCanaries bool
	// Required number of canaries to process.  Use 0 as 'no limit'.
	canaryLimit int
}

func NewUpgradeState(canaryInstances, allInstances []service.Instance, canaryLimit int) (*upgradeState, error) {
	us := upgradeState{}
	us.processCanaries = len(canaryInstances) > 0
	us.canaryLimit = canaryLimit
	us.states = map[string]instanceInfo{}
	for _, i := range allInstances {
		us.guids = append(us.guids, i.GUID)
		us.states[i.GUID] = instanceInfo{status: services.UpgradePending, initialPlan: i.PlanUniqueID}
	}
	for _, i := range canaryInstances {
		info, ok := us.states[i.GUID]
		if !ok {
			return nil, fmt.Errorf("Canary '%s' not in instance list", i.GUID)
		}
		info.couldBeCanary = true
		us.states[i.GUID] = info
	}
	return &us, nil
}

func (us *upgradeState) Next() (service.Instance, error) {
	for _, guid := range us.guids {
		if us.upgradeable(guid) {
			return service.Instance{GUID: guid, PlanUniqueID: us.states[guid].initialPlan}, nil
		}
	}
	return service.Instance{}, errors.New("Cannot retrieve next canary instance")
}

func (us *upgradeState) SetState(guid string, status services.UpgradeOperationType) error {
	info := us.states[guid]
	info.status = status
	us.states[guid] = info
	return nil
}

func (us *upgradeState) UpgradeCompleted() bool {
	if us.processCanaries {
		return us.canariesCompleted()
	}
	return us.allCompleted()
}

func (us *upgradeState) canariesCompleted() bool {
	completedCanaries := 0
	for _, info := range us.states {
		if !info.couldBeCanary {
			continue
		}
		if !isFinalState(info.status) && us.canaryLimit == 0 {
			return false
		}
		if isFinalState(info.status) {
			completedCanaries++
		}
	}
	return completedCanaries >= us.canaryLimit
}

func (us *upgradeState) allCompleted() bool {
	for _, info := range us.states {
		if !isFinalState(info.status) {
			return false
		}
	}
	return true
}

func (us *upgradeState) MarkCanariesCompleted() {
	us.processCanaries = false
}

func (us *upgradeState) upgradeable(guid string) bool {
	return us.doingCanariesAndPendingCanary(guid) ||
		us.notDoingCanariesAndPendingInstance(guid)
}

func (us *upgradeState) doingCanariesAndPendingCanary(guid string) bool {
	return us.processCanaries &&
		us.states[guid].couldBeCanary &&
		us.states[guid].status == services.UpgradePending
}

func (us *upgradeState) notDoingCanariesAndPendingInstance(guid string) bool {
	return !us.processCanaries &&
		us.states[guid].status == services.UpgradePending
}

func isFinalState(status services.UpgradeOperationType) bool {
	// TODO:
	// * add tests
	// * add missing states
	return status == services.UpgradeSucceeded || status == services.UpgradeFailed
}
