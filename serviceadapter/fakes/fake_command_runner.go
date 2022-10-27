// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

type FakeCommandRunner struct {
	RunStub        func(...string) ([]byte, []byte, *int, error)
	runMutex       sync.RWMutex
	runArgsForCall []struct {
		arg1 []string
	}
	runReturns struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}
	runReturnsOnCall map[int]struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}
	RunWithInputParamsStub        func(interface{}, ...string) ([]byte, []byte, *int, error)
	runWithInputParamsMutex       sync.RWMutex
	runWithInputParamsArgsForCall []struct {
		arg1 interface{}
		arg2 []string
	}
	runWithInputParamsReturns struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}
	runWithInputParamsReturnsOnCall map[int]struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCommandRunner) Run(arg1 ...string) ([]byte, []byte, *int, error) {
	fake.runMutex.Lock()
	ret, specificReturn := fake.runReturnsOnCall[len(fake.runArgsForCall)]
	fake.runArgsForCall = append(fake.runArgsForCall, struct {
		arg1 []string
	}{arg1})
	stub := fake.RunStub
	fakeReturns := fake.runReturns
	fake.recordInvocation("Run", []interface{}{arg1})
	fake.runMutex.Unlock()
	if stub != nil {
		return stub(arg1...)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3, ret.result4
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3, fakeReturns.result4
}

func (fake *FakeCommandRunner) RunCallCount() int {
	fake.runMutex.RLock()
	defer fake.runMutex.RUnlock()
	return len(fake.runArgsForCall)
}

func (fake *FakeCommandRunner) RunCalls(stub func(...string) ([]byte, []byte, *int, error)) {
	fake.runMutex.Lock()
	defer fake.runMutex.Unlock()
	fake.RunStub = stub
}

func (fake *FakeCommandRunner) RunArgsForCall(i int) []string {
	fake.runMutex.RLock()
	defer fake.runMutex.RUnlock()
	argsForCall := fake.runArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeCommandRunner) RunReturns(result1 []byte, result2 []byte, result3 *int, result4 error) {
	fake.runMutex.Lock()
	defer fake.runMutex.Unlock()
	fake.RunStub = nil
	fake.runReturns = struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeCommandRunner) RunReturnsOnCall(i int, result1 []byte, result2 []byte, result3 *int, result4 error) {
	fake.runMutex.Lock()
	defer fake.runMutex.Unlock()
	fake.RunStub = nil
	if fake.runReturnsOnCall == nil {
		fake.runReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 []byte
			result3 *int
			result4 error
		})
	}
	fake.runReturnsOnCall[i] = struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeCommandRunner) RunWithInputParams(arg1 interface{}, arg2 ...string) ([]byte, []byte, *int, error) {
	fake.runWithInputParamsMutex.Lock()
	ret, specificReturn := fake.runWithInputParamsReturnsOnCall[len(fake.runWithInputParamsArgsForCall)]
	fake.runWithInputParamsArgsForCall = append(fake.runWithInputParamsArgsForCall, struct {
		arg1 interface{}
		arg2 []string
	}{arg1, arg2})
	stub := fake.RunWithInputParamsStub
	fakeReturns := fake.runWithInputParamsReturns
	fake.recordInvocation("RunWithInputParams", []interface{}{arg1, arg2})
	fake.runWithInputParamsMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3, ret.result4
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3, fakeReturns.result4
}

func (fake *FakeCommandRunner) RunWithInputParamsCallCount() int {
	fake.runWithInputParamsMutex.RLock()
	defer fake.runWithInputParamsMutex.RUnlock()
	return len(fake.runWithInputParamsArgsForCall)
}

func (fake *FakeCommandRunner) RunWithInputParamsCalls(stub func(interface{}, ...string) ([]byte, []byte, *int, error)) {
	fake.runWithInputParamsMutex.Lock()
	defer fake.runWithInputParamsMutex.Unlock()
	fake.RunWithInputParamsStub = stub
}

func (fake *FakeCommandRunner) RunWithInputParamsArgsForCall(i int) (interface{}, []string) {
	fake.runWithInputParamsMutex.RLock()
	defer fake.runWithInputParamsMutex.RUnlock()
	argsForCall := fake.runWithInputParamsArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeCommandRunner) RunWithInputParamsReturns(result1 []byte, result2 []byte, result3 *int, result4 error) {
	fake.runWithInputParamsMutex.Lock()
	defer fake.runWithInputParamsMutex.Unlock()
	fake.RunWithInputParamsStub = nil
	fake.runWithInputParamsReturns = struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeCommandRunner) RunWithInputParamsReturnsOnCall(i int, result1 []byte, result2 []byte, result3 *int, result4 error) {
	fake.runWithInputParamsMutex.Lock()
	defer fake.runWithInputParamsMutex.Unlock()
	fake.RunWithInputParamsStub = nil
	if fake.runWithInputParamsReturnsOnCall == nil {
		fake.runWithInputParamsReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 []byte
			result3 *int
			result4 error
		})
	}
	fake.runWithInputParamsReturnsOnCall[i] = struct {
		result1 []byte
		result2 []byte
		result3 *int
		result4 error
	}{result1, result2, result3, result4}
}

func (fake *FakeCommandRunner) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.runMutex.RLock()
	defer fake.runMutex.RUnlock()
	fake.runWithInputParamsMutex.RLock()
	defer fake.runWithInputParamsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCommandRunner) recordInvocation(key string, args []interface{}) {
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

var _ serviceadapter.CommandRunner = new(FakeCommandRunner)
