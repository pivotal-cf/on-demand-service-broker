// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"log"
	"sync"

	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"
)

type FakeDecider struct {
	CanProvisionStub        func([]domain.Service, string, *domain.MaintenanceInfo, *log.Logger) error
	canProvisionMutex       sync.RWMutex
	canProvisionArgsForCall []struct {
		arg1 []domain.Service
		arg2 string
		arg3 *domain.MaintenanceInfo
		arg4 *log.Logger
	}
	canProvisionReturns struct {
		result1 error
	}
	canProvisionReturnsOnCall map[int]struct {
		result1 error
	}
	DecideOperationStub        func([]domain.Service, domain.UpdateDetails, *log.Logger) (decider.Operation, error)
	decideOperationMutex       sync.RWMutex
	decideOperationArgsForCall []struct {
		arg1 []domain.Service
		arg2 domain.UpdateDetails
		arg3 *log.Logger
	}
	decideOperationReturns struct {
		result1 decider.Operation
		result2 error
	}
	decideOperationReturnsOnCall map[int]struct {
		result1 decider.Operation
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeDecider) CanProvision(arg1 []domain.Service, arg2 string, arg3 *domain.MaintenanceInfo, arg4 *log.Logger) error {
	var arg1Copy []domain.Service
	if arg1 != nil {
		arg1Copy = make([]domain.Service, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.canProvisionMutex.Lock()
	ret, specificReturn := fake.canProvisionReturnsOnCall[len(fake.canProvisionArgsForCall)]
	fake.canProvisionArgsForCall = append(fake.canProvisionArgsForCall, struct {
		arg1 []domain.Service
		arg2 string
		arg3 *domain.MaintenanceInfo
		arg4 *log.Logger
	}{arg1Copy, arg2, arg3, arg4})
	stub := fake.CanProvisionStub
	fakeReturns := fake.canProvisionReturns
	fake.recordInvocation("CanProvision", []interface{}{arg1Copy, arg2, arg3, arg4})
	fake.canProvisionMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDecider) CanProvisionCallCount() int {
	fake.canProvisionMutex.RLock()
	defer fake.canProvisionMutex.RUnlock()
	return len(fake.canProvisionArgsForCall)
}

func (fake *FakeDecider) CanProvisionCalls(stub func([]domain.Service, string, *domain.MaintenanceInfo, *log.Logger) error) {
	fake.canProvisionMutex.Lock()
	defer fake.canProvisionMutex.Unlock()
	fake.CanProvisionStub = stub
}

func (fake *FakeDecider) CanProvisionArgsForCall(i int) ([]domain.Service, string, *domain.MaintenanceInfo, *log.Logger) {
	fake.canProvisionMutex.RLock()
	defer fake.canProvisionMutex.RUnlock()
	argsForCall := fake.canProvisionArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeDecider) CanProvisionReturns(result1 error) {
	fake.canProvisionMutex.Lock()
	defer fake.canProvisionMutex.Unlock()
	fake.CanProvisionStub = nil
	fake.canProvisionReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDecider) CanProvisionReturnsOnCall(i int, result1 error) {
	fake.canProvisionMutex.Lock()
	defer fake.canProvisionMutex.Unlock()
	fake.CanProvisionStub = nil
	if fake.canProvisionReturnsOnCall == nil {
		fake.canProvisionReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.canProvisionReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeDecider) DecideOperation(arg1 []domain.Service, arg2 domain.UpdateDetails, arg3 *log.Logger) (decider.Operation, error) {
	var arg1Copy []domain.Service
	if arg1 != nil {
		arg1Copy = make([]domain.Service, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.decideOperationMutex.Lock()
	ret, specificReturn := fake.decideOperationReturnsOnCall[len(fake.decideOperationArgsForCall)]
	fake.decideOperationArgsForCall = append(fake.decideOperationArgsForCall, struct {
		arg1 []domain.Service
		arg2 domain.UpdateDetails
		arg3 *log.Logger
	}{arg1Copy, arg2, arg3})
	stub := fake.DecideOperationStub
	fakeReturns := fake.decideOperationReturns
	fake.recordInvocation("DecideOperation", []interface{}{arg1Copy, arg2, arg3})
	fake.decideOperationMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDecider) DecideOperationCallCount() int {
	fake.decideOperationMutex.RLock()
	defer fake.decideOperationMutex.RUnlock()
	return len(fake.decideOperationArgsForCall)
}

func (fake *FakeDecider) DecideOperationCalls(stub func([]domain.Service, domain.UpdateDetails, *log.Logger) (decider.Operation, error)) {
	fake.decideOperationMutex.Lock()
	defer fake.decideOperationMutex.Unlock()
	fake.DecideOperationStub = stub
}

func (fake *FakeDecider) DecideOperationArgsForCall(i int) ([]domain.Service, domain.UpdateDetails, *log.Logger) {
	fake.decideOperationMutex.RLock()
	defer fake.decideOperationMutex.RUnlock()
	argsForCall := fake.decideOperationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeDecider) DecideOperationReturns(result1 decider.Operation, result2 error) {
	fake.decideOperationMutex.Lock()
	defer fake.decideOperationMutex.Unlock()
	fake.DecideOperationStub = nil
	fake.decideOperationReturns = struct {
		result1 decider.Operation
		result2 error
	}{result1, result2}
}

func (fake *FakeDecider) DecideOperationReturnsOnCall(i int, result1 decider.Operation, result2 error) {
	fake.decideOperationMutex.Lock()
	defer fake.decideOperationMutex.Unlock()
	fake.DecideOperationStub = nil
	if fake.decideOperationReturnsOnCall == nil {
		fake.decideOperationReturnsOnCall = make(map[int]struct {
			result1 decider.Operation
			result2 error
		})
	}
	fake.decideOperationReturnsOnCall[i] = struct {
		result1 decider.Operation
		result2 error
	}{result1, result2}
}

func (fake *FakeDecider) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.canProvisionMutex.RLock()
	defer fake.canProvisionMutex.RUnlock()
	fake.decideOperationMutex.RLock()
	defer fake.decideOperationMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeDecider) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ broker.Decider = new(FakeDecider)
