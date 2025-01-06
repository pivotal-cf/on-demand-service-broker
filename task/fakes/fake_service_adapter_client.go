// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"log"
	"sync"

	"github.com/pivotal-cf/brokerapi/v12/domain"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type FakeServiceAdapterClient struct {
	GenerateManifestStub        func(serviceadapter.ServiceDeployment, serviceadapter.Plan, map[string]interface{}, []byte, *serviceadapter.Plan, map[string]string, map[string]string, map[string]string, *log.Logger) (serviceadapter.MarshalledGenerateManifest, error)
	generateManifestMutex       sync.RWMutex
	generateManifestArgsForCall []struct {
		arg1 serviceadapter.ServiceDeployment
		arg2 serviceadapter.Plan
		arg3 map[string]interface{}
		arg4 []byte
		arg5 *serviceadapter.Plan
		arg6 map[string]string
		arg7 map[string]string
		arg8 map[string]string
		arg9 *log.Logger
	}
	generateManifestReturns struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}
	generateManifestReturnsOnCall map[int]struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}
	GeneratePlanSchemaStub        func(serviceadapter.Plan, *log.Logger) (domain.ServiceSchemas, error)
	generatePlanSchemaMutex       sync.RWMutex
	generatePlanSchemaArgsForCall []struct {
		arg1 serviceadapter.Plan
		arg2 *log.Logger
	}
	generatePlanSchemaReturns struct {
		result1 domain.ServiceSchemas
		result2 error
	}
	generatePlanSchemaReturnsOnCall map[int]struct {
		result1 domain.ServiceSchemas
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeServiceAdapterClient) GenerateManifest(arg1 serviceadapter.ServiceDeployment, arg2 serviceadapter.Plan, arg3 map[string]interface{}, arg4 []byte, arg5 *serviceadapter.Plan, arg6 map[string]string, arg7 map[string]string, arg8 map[string]string, arg9 *log.Logger) (serviceadapter.MarshalledGenerateManifest, error) {
	var arg4Copy []byte
	if arg4 != nil {
		arg4Copy = make([]byte, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.generateManifestMutex.Lock()
	ret, specificReturn := fake.generateManifestReturnsOnCall[len(fake.generateManifestArgsForCall)]
	fake.generateManifestArgsForCall = append(fake.generateManifestArgsForCall, struct {
		arg1 serviceadapter.ServiceDeployment
		arg2 serviceadapter.Plan
		arg3 map[string]interface{}
		arg4 []byte
		arg5 *serviceadapter.Plan
		arg6 map[string]string
		arg7 map[string]string
		arg8 map[string]string
		arg9 *log.Logger
	}{arg1, arg2, arg3, arg4Copy, arg5, arg6, arg7, arg8, arg9})
	stub := fake.GenerateManifestStub
	fakeReturns := fake.generateManifestReturns
	fake.recordInvocation("GenerateManifest", []interface{}{arg1, arg2, arg3, arg4Copy, arg5, arg6, arg7, arg8, arg9})
	fake.generateManifestMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeServiceAdapterClient) GenerateManifestCallCount() int {
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	return len(fake.generateManifestArgsForCall)
}

func (fake *FakeServiceAdapterClient) GenerateManifestCalls(stub func(serviceadapter.ServiceDeployment, serviceadapter.Plan, map[string]interface{}, []byte, *serviceadapter.Plan, map[string]string, map[string]string, map[string]string, *log.Logger) (serviceadapter.MarshalledGenerateManifest, error)) {
	fake.generateManifestMutex.Lock()
	defer fake.generateManifestMutex.Unlock()
	fake.GenerateManifestStub = stub
}

func (fake *FakeServiceAdapterClient) GenerateManifestArgsForCall(i int) (serviceadapter.ServiceDeployment, serviceadapter.Plan, map[string]interface{}, []byte, *serviceadapter.Plan, map[string]string, map[string]string, map[string]string, *log.Logger) {
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	argsForCall := fake.generateManifestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5, argsForCall.arg6, argsForCall.arg7, argsForCall.arg8, argsForCall.arg9
}

func (fake *FakeServiceAdapterClient) GenerateManifestReturns(result1 serviceadapter.MarshalledGenerateManifest, result2 error) {
	fake.generateManifestMutex.Lock()
	defer fake.generateManifestMutex.Unlock()
	fake.GenerateManifestStub = nil
	fake.generateManifestReturns = struct {
		result1 serviceadapter.MarshalledGenerateManifest
		result2 error
	}{result1, result2}
}

func (fake *FakeServiceAdapterClient) GenerateManifestReturnsOnCall(i int, result1 serviceadapter.MarshalledGenerateManifest, result2 error) {
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

func (fake *FakeServiceAdapterClient) GeneratePlanSchema(arg1 serviceadapter.Plan, arg2 *log.Logger) (domain.ServiceSchemas, error) {
	fake.generatePlanSchemaMutex.Lock()
	ret, specificReturn := fake.generatePlanSchemaReturnsOnCall[len(fake.generatePlanSchemaArgsForCall)]
	fake.generatePlanSchemaArgsForCall = append(fake.generatePlanSchemaArgsForCall, struct {
		arg1 serviceadapter.Plan
		arg2 *log.Logger
	}{arg1, arg2})
	stub := fake.GeneratePlanSchemaStub
	fakeReturns := fake.generatePlanSchemaReturns
	fake.recordInvocation("GeneratePlanSchema", []interface{}{arg1, arg2})
	fake.generatePlanSchemaMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeServiceAdapterClient) GeneratePlanSchemaCallCount() int {
	fake.generatePlanSchemaMutex.RLock()
	defer fake.generatePlanSchemaMutex.RUnlock()
	return len(fake.generatePlanSchemaArgsForCall)
}

func (fake *FakeServiceAdapterClient) GeneratePlanSchemaCalls(stub func(serviceadapter.Plan, *log.Logger) (domain.ServiceSchemas, error)) {
	fake.generatePlanSchemaMutex.Lock()
	defer fake.generatePlanSchemaMutex.Unlock()
	fake.GeneratePlanSchemaStub = stub
}

func (fake *FakeServiceAdapterClient) GeneratePlanSchemaArgsForCall(i int) (serviceadapter.Plan, *log.Logger) {
	fake.generatePlanSchemaMutex.RLock()
	defer fake.generatePlanSchemaMutex.RUnlock()
	argsForCall := fake.generatePlanSchemaArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeServiceAdapterClient) GeneratePlanSchemaReturns(result1 domain.ServiceSchemas, result2 error) {
	fake.generatePlanSchemaMutex.Lock()
	defer fake.generatePlanSchemaMutex.Unlock()
	fake.GeneratePlanSchemaStub = nil
	fake.generatePlanSchemaReturns = struct {
		result1 domain.ServiceSchemas
		result2 error
	}{result1, result2}
}

func (fake *FakeServiceAdapterClient) GeneratePlanSchemaReturnsOnCall(i int, result1 domain.ServiceSchemas, result2 error) {
	fake.generatePlanSchemaMutex.Lock()
	defer fake.generatePlanSchemaMutex.Unlock()
	fake.GeneratePlanSchemaStub = nil
	if fake.generatePlanSchemaReturnsOnCall == nil {
		fake.generatePlanSchemaReturnsOnCall = make(map[int]struct {
			result1 domain.ServiceSchemas
			result2 error
		})
	}
	fake.generatePlanSchemaReturnsOnCall[i] = struct {
		result1 domain.ServiceSchemas
		result2 error
	}{result1, result2}
}

func (fake *FakeServiceAdapterClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateManifestMutex.RLock()
	defer fake.generateManifestMutex.RUnlock()
	fake.generatePlanSchemaMutex.RLock()
	defer fake.generatePlanSchemaMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeServiceAdapterClient) recordInvocation(key string, args []interface{}) {
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

var _ task.ServiceAdapterClient = new(FakeServiceAdapterClient)
