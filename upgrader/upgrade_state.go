package upgrader

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type instanceInfo struct {
	status           services.UpgradeOperationType
	initialPlan      string
	upgradeOperation services.UpgradeOperation
	couldBeCanary    bool
}

type upgradeState struct {
	guids           []string
	states          map[string]instanceInfo
	processCanaries bool
	// Required number of canaries to process.  Use 0 as 'no limit'.
	canaryLimit int
	pos         int
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

func (us *upgradeState) IsProcessingCanaries() bool {
	return us.processCanaries
}

func (us *upgradeState) Retry() {
	us.pos = 0
	for k, v := range us.states {
		if v.status == services.OperationInProgress {
			v.status = services.UpgradePending
			us.states[k] = v
		}
	}
}

func (us *upgradeState) RetryBusyInstances() {
	us.pos = 0
}

func (us *upgradeState) NextPending() (service.Instance, error) {
	for us.pos < len(us.guids) {
		guid := us.guids[us.pos]
		us.pos++
		if us.upgradeable(guid) {
			return service.Instance{GUID: guid, PlanUniqueID: us.states[guid].initialPlan}, nil
		}
	}
	return service.Instance{}, errors.New("Cannot retrieve next pending instance")
}

func (us *upgradeState) GetUpgradeIndex() int {
	return len(us.GetInstancesInStates(services.UpgradeSucceeded, services.UpgradeAccepted, services.InstanceNotFound, services.OrphanDeployment)) + 1
}

func (us *upgradeState) GetGUIDsInStates(states ...services.UpgradeOperationType) (guids []string) {
	guids = []string{}
	for _, i := range us.GetInstancesInStates(states...) {
		guids = append(guids, i.GUID)
	}
	return
}

func (us *upgradeState) GetInstancesInStates(states ...services.UpgradeOperationType) (instances []service.Instance) {
	instances = []service.Instance{}
	for _, guid := range us.guids {
		inst := us.states[guid]
		if us.processCanaries && !inst.couldBeCanary {
			continue
		}
		for _, state := range states {
			if inst.status == state {
				instances = append(instances, service.Instance{GUID: guid, PlanUniqueID: inst.initialPlan})
			}
		}
	}
	return
}

func (us *upgradeState) SetState(guid string, status services.UpgradeOperationType) error {
	info := us.states[guid]
	info.status = status
	us.states[guid] = info
	return nil
}

func (us *upgradeState) SetUpgradeOperation(guid string, upgradeOp services.UpgradeOperation) {
	info := us.states[guid]
	info.upgradeOperation = upgradeOp
	us.states[guid] = info
}

func (us *upgradeState) GetUpgradeOperation(guid string) services.UpgradeOperation {
	return us.states[guid].upgradeOperation
}

func (us *upgradeState) PhaseComplete() bool {
	if us.processCanaries {
		return us.canariesCompleted()
	}
	return us.allCompleted()
}

func (us *upgradeState) OutstandingCanaryCount() int {
	pending := 0
	triggered := 0

	for _, guid := range us.guids {
		info := us.states[guid]
		if !info.couldBeCanary {
			continue
		}
		if info.status == services.UpgradePending {
			pending++
		} else {
			triggered++
		}
	}

	outstanding := pending
	if us.canaryLimit > 0 {
		outstanding = us.canaryLimit - triggered
	}

	return outstanding
}

func (us *upgradeState) DebugStates() {
	for guid, info := range us.states {
		fmt.Printf("%s: %s\n", guid, info.status)
	}
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
	us.pos = 0
}

func (us *upgradeState) InstanceCountInPhase() int {
	c := 0
	for _, inst := range us.states {
		if us.processCanaries && !inst.couldBeCanary {
			continue
		}
		c++
	}
	return c
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
	return status != services.OperationInProgress && status != services.UpgradePending && status != services.UpgradeAccepted //status == services.UpgradeSucceeded || status == services.UpgradeFailed
}
