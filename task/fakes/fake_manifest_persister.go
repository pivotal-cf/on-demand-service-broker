// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/task"
)

type FakeManifestPersister struct {
	PersistManifestStub        func(string, string, []byte)
	persistManifestMutex       sync.RWMutex
	persistManifestArgsForCall []struct {
		arg1 string
		arg2 string
		arg3 []byte
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeManifestPersister) PersistManifest(arg1 string, arg2 string, arg3 []byte) {
	var arg3Copy []byte
	if arg3 != nil {
		arg3Copy = make([]byte, len(arg3))
		copy(arg3Copy, arg3)
	}
	fake.persistManifestMutex.Lock()
	fake.persistManifestArgsForCall = append(fake.persistManifestArgsForCall, struct {
		arg1 string
		arg2 string
		arg3 []byte
	}{arg1, arg2, arg3Copy})
	stub := fake.PersistManifestStub
	fake.recordInvocation("PersistManifest", []interface{}{arg1, arg2, arg3Copy})
	fake.persistManifestMutex.Unlock()
	if stub != nil {
		fake.PersistManifestStub(arg1, arg2, arg3)
	}
}

func (fake *FakeManifestPersister) PersistManifestCallCount() int {
	fake.persistManifestMutex.RLock()
	defer fake.persistManifestMutex.RUnlock()
	return len(fake.persistManifestArgsForCall)
}

func (fake *FakeManifestPersister) PersistManifestCalls(stub func(string, string, []byte)) {
	fake.persistManifestMutex.Lock()
	defer fake.persistManifestMutex.Unlock()
	fake.PersistManifestStub = stub
}

func (fake *FakeManifestPersister) PersistManifestArgsForCall(i int) (string, string, []byte) {
	fake.persistManifestMutex.RLock()
	defer fake.persistManifestMutex.RUnlock()
	argsForCall := fake.persistManifestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeManifestPersister) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.persistManifestMutex.RLock()
	defer fake.persistManifestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeManifestPersister) recordInvocation(key string, args []interface{}) {
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

var _ task.ManifestPersister = new(FakeManifestPersister)
