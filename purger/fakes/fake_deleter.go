// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/purger"
)

type FakeDeleter struct {
	DeleteAllServiceInstancesStub        func(string) error
	deleteAllServiceInstancesMutex       sync.RWMutex
	deleteAllServiceInstancesArgsForCall []struct {
		arg1 string
	}
	deleteAllServiceInstancesReturns struct {
		result1 error
	}
	deleteAllServiceInstancesReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeDeleter) DeleteAllServiceInstances(arg1 string) error {
	fake.deleteAllServiceInstancesMutex.Lock()
	ret, specificReturn := fake.deleteAllServiceInstancesReturnsOnCall[len(fake.deleteAllServiceInstancesArgsForCall)]
	fake.deleteAllServiceInstancesArgsForCall = append(fake.deleteAllServiceInstancesArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.DeleteAllServiceInstancesStub
	fakeReturns := fake.deleteAllServiceInstancesReturns
	fake.recordInvocation("DeleteAllServiceInstances", []interface{}{arg1})
	fake.deleteAllServiceInstancesMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDeleter) DeleteAllServiceInstancesCallCount() int {
	fake.deleteAllServiceInstancesMutex.RLock()
	defer fake.deleteAllServiceInstancesMutex.RUnlock()
	return len(fake.deleteAllServiceInstancesArgsForCall)
}

func (fake *FakeDeleter) DeleteAllServiceInstancesCalls(stub func(string) error) {
	fake.deleteAllServiceInstancesMutex.Lock()
	defer fake.deleteAllServiceInstancesMutex.Unlock()
	fake.DeleteAllServiceInstancesStub = stub
}

func (fake *FakeDeleter) DeleteAllServiceInstancesArgsForCall(i int) string {
	fake.deleteAllServiceInstancesMutex.RLock()
	defer fake.deleteAllServiceInstancesMutex.RUnlock()
	argsForCall := fake.deleteAllServiceInstancesArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDeleter) DeleteAllServiceInstancesReturns(result1 error) {
	fake.deleteAllServiceInstancesMutex.Lock()
	defer fake.deleteAllServiceInstancesMutex.Unlock()
	fake.DeleteAllServiceInstancesStub = nil
	fake.deleteAllServiceInstancesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDeleter) DeleteAllServiceInstancesReturnsOnCall(i int, result1 error) {
	fake.deleteAllServiceInstancesMutex.Lock()
	defer fake.deleteAllServiceInstancesMutex.Unlock()
	fake.DeleteAllServiceInstancesStub = nil
	if fake.deleteAllServiceInstancesReturnsOnCall == nil {
		fake.deleteAllServiceInstancesReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteAllServiceInstancesReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeDeleter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.deleteAllServiceInstancesMutex.RLock()
	defer fake.deleteAllServiceInstancesMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeDeleter) recordInvocation(key string, args []interface{}) {
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

var _ purger.Deleter = new(FakeDeleter)
