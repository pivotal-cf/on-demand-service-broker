// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"log"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type FakeManifestGenerator struct {
	GenerateManifestStub        func(task.GenerateManifestProperties, *log.Logger) (serviceadapter.MarshalledGenerateManifest, error)
	generateManifestMutex       sync.RWMutex
	generateManifestArgsForCall []struct {
		arg1 task.GenerateManifestProperties
		arg2 *log.Logger
	}
	generateManifestReturns struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}
	generateManifestReturnsOnCall map[int]struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeManifestGenerator) GenerateManifest(arg1 task.GenerateManifestProperties, arg2 *log.Logger) (serviceadapter.MarshalledGenerateManifest, error) {
	fake.generateManifestMutex.Lock()
	ret, specificReturn := fake.generateManifestReturnsOnCall[len(fake.generateManifestArgsForCall)]
	fake.generateManifestArgsForCall = append(fake.generateManifestArgsForCall, struct {
		arg1 task.GenerateManifestProperties
		arg2 *log.Logger
	}{arg1, arg2})
	stub := fake.GenerateManifestStub
	fakeReturns := fake.generateManifestReturns
	fake.recordInvocation("GenerateManifest", []interface{}{arg1, arg2})
	fake.generateManifestMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeManifestGenerator) GenerateManifestCallCount() int {
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	return len(fake.generateManifestArgsForCall)
}

func (fake *FakeManifestGenerator) GenerateManifestCalls(stub func(task.GenerateManifestProperties, *log.Logger) (serviceadapter.MarshalledGenerateManifest, error)) {
	fake.generateManifestMutex.Lock()
	defer fake.generateManifestMutex.Unlock()
	fake.GenerateManifestStub = stub
}

func (fake *FakeManifestGenerator) GenerateManifestArgsForCall(i int) (task.GenerateManifestProperties, *log.Logger) {
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	argsForCall := fake.generateManifestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeManifestGenerator) GenerateManifestReturns(result1 serviceadapter.MarshalledGenerateManifest, result2 error) {
	fake.generateManifestMutex.Lock()
	defer fake.generateManifestMutex.Unlock()
	fake.GenerateManifestStub = nil
	fake.generateManifestReturns = struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}{result1, result2}
}

func (fake *FakeManifestGenerator) GenerateManifestReturnsOnCall(i int, result1 serviceadapter.MarshalledGenerateManifest, result2 error) {
	fake.generateManifestMutex.Lock()
	defer fake.generateManifestMutex.Unlock()
	fake.GenerateManifestStub = nil
	if fake.generateManifestReturnsOnCall == nil {
		fake.generateManifestReturnsOnCall = make(map[int]struct {
			result1 serviceadapter.MarshalledGenerateManifest
			result2 error
		})
	}
	fake.generateManifestReturnsOnCall[i] = struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}{result1, result2}
}

func (fake *FakeManifestGenerator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeManifestGenerator) recordInvocation(key string, args []interface{}) {
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

var _ task.ManifestGenerator = new(FakeManifestGenerator)
