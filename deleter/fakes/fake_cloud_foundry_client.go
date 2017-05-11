// This file was generated by counterfeiter
package fakes

import (
	"log"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
)

type FakeCloudFoundryClient struct {
	GetInstancesOfServiceOfferingStub        func(serviceOfferingID string, logger *log.Logger) ([]string, error)
	getInstancesOfServiceOfferingMutex       sync.RWMutex
	getInstancesOfServiceOfferingArgsForCall []struct {
		serviceOfferingID string
		logger            *log.Logger
	}
	getInstancesOfServiceOfferingReturns struct {
		result1 []string
		result2 error
	}
	getInstancesOfServiceOfferingReturnsOnCall map[int]struct {
		result1 []string
		result2 error
	}
	GetInstanceStub        func(instanceGUID string, logger *log.Logger) (cf.Instance, error)
	getInstanceMutex       sync.RWMutex
	getInstanceArgsForCall []struct {
		instanceGUID string
		logger       *log.Logger
	}
	getInstanceReturns struct {
		result1 cf.Instance
		result2 error
	}
	getInstanceReturnsOnCall map[int]struct {
		result1 cf.Instance
		result2 error
	}
	GetBindingsForInstanceStub        func(instanceGUID string, logger *log.Logger) ([]cf.Binding, error)
	getBindingsForInstanceMutex       sync.RWMutex
	getBindingsForInstanceArgsForCall []struct {
		instanceGUID string
		logger       *log.Logger
	}
	getBindingsForInstanceReturns struct {
		result1 []cf.Binding
		result2 error
	}
	getBindingsForInstanceReturnsOnCall map[int]struct {
		result1 []cf.Binding
		result2 error
	}
	DeleteBindingStub        func(binding cf.Binding, logger *log.Logger) error
	deleteBindingMutex       sync.RWMutex
	deleteBindingArgsForCall []struct {
		binding cf.Binding
		logger  *log.Logger
	}
	deleteBindingReturns struct {
		result1 error
	}
	deleteBindingReturnsOnCall map[int]struct {
		result1 error
	}
	GetServiceKeysForInstanceStub        func(instanceGUID string, logger *log.Logger) ([]cf.ServiceKey, error)
	getServiceKeysForInstanceMutex       sync.RWMutex
	getServiceKeysForInstanceArgsForCall []struct {
		instanceGUID string
		logger       *log.Logger
	}
	getServiceKeysForInstanceReturns struct {
		result1 []cf.ServiceKey
		result2 error
	}
	getServiceKeysForInstanceReturnsOnCall map[int]struct {
		result1 []cf.ServiceKey
		result2 error
	}
	DeleteServiceKeyStub        func(serviceKey cf.ServiceKey, logger *log.Logger) error
	deleteServiceKeyMutex       sync.RWMutex
	deleteServiceKeyArgsForCall []struct {
		serviceKey cf.ServiceKey
		logger     *log.Logger
	}
	deleteServiceKeyReturns struct {
		result1 error
	}
	deleteServiceKeyReturnsOnCall map[int]struct {
		result1 error
	}
	DeleteServiceInstanceStub        func(instanceGUID string, logger *log.Logger) error
	deleteServiceInstanceMutex       sync.RWMutex
	deleteServiceInstanceArgsForCall []struct {
		instanceGUID string
		logger       *log.Logger
	}
	deleteServiceInstanceReturns struct {
		result1 error
	}
	deleteServiceInstanceReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCloudFoundryClient) GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]string, error) {
	fake.getInstancesOfServiceOfferingMutex.Lock()
	ret, specificReturn := fake.getInstancesOfServiceOfferingReturnsOnCall[len(fake.getInstancesOfServiceOfferingArgsForCall)]
	fake.getInstancesOfServiceOfferingArgsForCall = append(fake.getInstancesOfServiceOfferingArgsForCall, struct {
		serviceOfferingID string
		logger            *log.Logger
	}{serviceOfferingID, logger})
	fake.recordInvocation("GetInstancesOfServiceOffering", []interface{}{serviceOfferingID, logger})
	fake.getInstancesOfServiceOfferingMutex.Unlock()
	if fake.GetInstancesOfServiceOfferingStub != nil {
		return fake.GetInstancesOfServiceOfferingStub(serviceOfferingID, logger)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getInstancesOfServiceOfferingReturns.result1, fake.getInstancesOfServiceOfferingReturns.result2
}

func (fake *FakeCloudFoundryClient) GetInstancesOfServiceOfferingCallCount() int {
	fake.getInstancesOfServiceOfferingMutex.RLock()
	defer fake.getInstancesOfServiceOfferingMutex.RUnlock()
	return len(fake.getInstancesOfServiceOfferingArgsForCall)
}

func (fake *FakeCloudFoundryClient) GetInstancesOfServiceOfferingArgsForCall(i int) (string, *log.Logger) {
	fake.getInstancesOfServiceOfferingMutex.RLock()
	defer fake.getInstancesOfServiceOfferingMutex.RUnlock()
	return fake.getInstancesOfServiceOfferingArgsForCall[i].serviceOfferingID, fake.getInstancesOfServiceOfferingArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) GetInstancesOfServiceOfferingReturns(result1 []string, result2 error) {
	fake.GetInstancesOfServiceOfferingStub = nil
	fake.getInstancesOfServiceOfferingReturns = struct {
		result1 []string
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetInstancesOfServiceOfferingReturnsOnCall(i int, result1 []string, result2 error) {
	fake.GetInstancesOfServiceOfferingStub = nil
	if fake.getInstancesOfServiceOfferingReturnsOnCall == nil {
		fake.getInstancesOfServiceOfferingReturnsOnCall = make(map[int]struct {
			result1 []string
			result2 error
		})
	}
	fake.getInstancesOfServiceOfferingReturnsOnCall[i] = struct {
		result1 []string
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetInstance(instanceGUID string, logger *log.Logger) (cf.Instance, error) {
	fake.getInstanceMutex.Lock()
	ret, specificReturn := fake.getInstanceReturnsOnCall[len(fake.getInstanceArgsForCall)]
	fake.getInstanceArgsForCall = append(fake.getInstanceArgsForCall, struct {
		instanceGUID string
		logger       *log.Logger
	}{instanceGUID, logger})
	fake.recordInvocation("GetInstance", []interface{}{instanceGUID, logger})
	fake.getInstanceMutex.Unlock()
	if fake.GetInstanceStub != nil {
		return fake.GetInstanceStub(instanceGUID, logger)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getInstanceReturns.result1, fake.getInstanceReturns.result2
}

func (fake *FakeCloudFoundryClient) GetInstanceCallCount() int {
	fake.getInstanceMutex.RLock()
	defer fake.getInstanceMutex.RUnlock()
	return len(fake.getInstanceArgsForCall)
}

func (fake *FakeCloudFoundryClient) GetInstanceArgsForCall(i int) (string, *log.Logger) {
	fake.getInstanceMutex.RLock()
	defer fake.getInstanceMutex.RUnlock()
	return fake.getInstanceArgsForCall[i].instanceGUID, fake.getInstanceArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) GetInstanceReturns(result1 cf.Instance, result2 error) {
	fake.GetInstanceStub = nil
	fake.getInstanceReturns = struct {
		result1 cf.Instance
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetInstanceReturnsOnCall(i int, result1 cf.Instance, result2 error) {
	fake.GetInstanceStub = nil
	if fake.getInstanceReturnsOnCall == nil {
		fake.getInstanceReturnsOnCall = make(map[int]struct {
			result1 cf.Instance
			result2 error
		})
	}
	fake.getInstanceReturnsOnCall[i] = struct {
		result1 cf.Instance
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetBindingsForInstance(instanceGUID string, logger *log.Logger) ([]cf.Binding, error) {
	fake.getBindingsForInstanceMutex.Lock()
	ret, specificReturn := fake.getBindingsForInstanceReturnsOnCall[len(fake.getBindingsForInstanceArgsForCall)]
	fake.getBindingsForInstanceArgsForCall = append(fake.getBindingsForInstanceArgsForCall, struct {
		instanceGUID string
		logger       *log.Logger
	}{instanceGUID, logger})
	fake.recordInvocation("GetBindingsForInstance", []interface{}{instanceGUID, logger})
	fake.getBindingsForInstanceMutex.Unlock()
	if fake.GetBindingsForInstanceStub != nil {
		return fake.GetBindingsForInstanceStub(instanceGUID, logger)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getBindingsForInstanceReturns.result1, fake.getBindingsForInstanceReturns.result2
}

func (fake *FakeCloudFoundryClient) GetBindingsForInstanceCallCount() int {
	fake.getBindingsForInstanceMutex.RLock()
	defer fake.getBindingsForInstanceMutex.RUnlock()
	return len(fake.getBindingsForInstanceArgsForCall)
}

func (fake *FakeCloudFoundryClient) GetBindingsForInstanceArgsForCall(i int) (string, *log.Logger) {
	fake.getBindingsForInstanceMutex.RLock()
	defer fake.getBindingsForInstanceMutex.RUnlock()
	return fake.getBindingsForInstanceArgsForCall[i].instanceGUID, fake.getBindingsForInstanceArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) GetBindingsForInstanceReturns(result1 []cf.Binding, result2 error) {
	fake.GetBindingsForInstanceStub = nil
	fake.getBindingsForInstanceReturns = struct {
		result1 []cf.Binding
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetBindingsForInstanceReturnsOnCall(i int, result1 []cf.Binding, result2 error) {
	fake.GetBindingsForInstanceStub = nil
	if fake.getBindingsForInstanceReturnsOnCall == nil {
		fake.getBindingsForInstanceReturnsOnCall = make(map[int]struct {
			result1 []cf.Binding
			result2 error
		})
	}
	fake.getBindingsForInstanceReturnsOnCall[i] = struct {
		result1 []cf.Binding
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) DeleteBinding(binding cf.Binding, logger *log.Logger) error {
	fake.deleteBindingMutex.Lock()
	ret, specificReturn := fake.deleteBindingReturnsOnCall[len(fake.deleteBindingArgsForCall)]
	fake.deleteBindingArgsForCall = append(fake.deleteBindingArgsForCall, struct {
		binding cf.Binding
		logger  *log.Logger
	}{binding, logger})
	fake.recordInvocation("DeleteBinding", []interface{}{binding, logger})
	fake.deleteBindingMutex.Unlock()
	if fake.DeleteBindingStub != nil {
		return fake.DeleteBindingStub(binding, logger)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.deleteBindingReturns.result1
}

func (fake *FakeCloudFoundryClient) DeleteBindingCallCount() int {
	fake.deleteBindingMutex.RLock()
	defer fake.deleteBindingMutex.RUnlock()
	return len(fake.deleteBindingArgsForCall)
}

func (fake *FakeCloudFoundryClient) DeleteBindingArgsForCall(i int) (cf.Binding, *log.Logger) {
	fake.deleteBindingMutex.RLock()
	defer fake.deleteBindingMutex.RUnlock()
	return fake.deleteBindingArgsForCall[i].binding, fake.deleteBindingArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) DeleteBindingReturns(result1 error) {
	fake.DeleteBindingStub = nil
	fake.deleteBindingReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) DeleteBindingReturnsOnCall(i int, result1 error) {
	fake.DeleteBindingStub = nil
	if fake.deleteBindingReturnsOnCall == nil {
		fake.deleteBindingReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteBindingReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) GetServiceKeysForInstance(instanceGUID string, logger *log.Logger) ([]cf.ServiceKey, error) {
	fake.getServiceKeysForInstanceMutex.Lock()
	ret, specificReturn := fake.getServiceKeysForInstanceReturnsOnCall[len(fake.getServiceKeysForInstanceArgsForCall)]
	fake.getServiceKeysForInstanceArgsForCall = append(fake.getServiceKeysForInstanceArgsForCall, struct {
		instanceGUID string
		logger       *log.Logger
	}{instanceGUID, logger})
	fake.recordInvocation("GetServiceKeysForInstance", []interface{}{instanceGUID, logger})
	fake.getServiceKeysForInstanceMutex.Unlock()
	if fake.GetServiceKeysForInstanceStub != nil {
		return fake.GetServiceKeysForInstanceStub(instanceGUID, logger)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.getServiceKeysForInstanceReturns.result1, fake.getServiceKeysForInstanceReturns.result2
}

func (fake *FakeCloudFoundryClient) GetServiceKeysForInstanceCallCount() int {
	fake.getServiceKeysForInstanceMutex.RLock()
	defer fake.getServiceKeysForInstanceMutex.RUnlock()
	return len(fake.getServiceKeysForInstanceArgsForCall)
}

func (fake *FakeCloudFoundryClient) GetServiceKeysForInstanceArgsForCall(i int) (string, *log.Logger) {
	fake.getServiceKeysForInstanceMutex.RLock()
	defer fake.getServiceKeysForInstanceMutex.RUnlock()
	return fake.getServiceKeysForInstanceArgsForCall[i].instanceGUID, fake.getServiceKeysForInstanceArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) GetServiceKeysForInstanceReturns(result1 []cf.ServiceKey, result2 error) {
	fake.GetServiceKeysForInstanceStub = nil
	fake.getServiceKeysForInstanceReturns = struct {
		result1 []cf.ServiceKey
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) GetServiceKeysForInstanceReturnsOnCall(i int, result1 []cf.ServiceKey, result2 error) {
	fake.GetServiceKeysForInstanceStub = nil
	if fake.getServiceKeysForInstanceReturnsOnCall == nil {
		fake.getServiceKeysForInstanceReturnsOnCall = make(map[int]struct {
			result1 []cf.ServiceKey
			result2 error
		})
	}
	fake.getServiceKeysForInstanceReturnsOnCall[i] = struct {
		result1 []cf.ServiceKey
		result2 error
	}{result1, result2}
}

func (fake *FakeCloudFoundryClient) DeleteServiceKey(serviceKey cf.ServiceKey, logger *log.Logger) error {
	fake.deleteServiceKeyMutex.Lock()
	ret, specificReturn := fake.deleteServiceKeyReturnsOnCall[len(fake.deleteServiceKeyArgsForCall)]
	fake.deleteServiceKeyArgsForCall = append(fake.deleteServiceKeyArgsForCall, struct {
		serviceKey cf.ServiceKey
		logger     *log.Logger
	}{serviceKey, logger})
	fake.recordInvocation("DeleteServiceKey", []interface{}{serviceKey, logger})
	fake.deleteServiceKeyMutex.Unlock()
	if fake.DeleteServiceKeyStub != nil {
		return fake.DeleteServiceKeyStub(serviceKey, logger)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.deleteServiceKeyReturns.result1
}

func (fake *FakeCloudFoundryClient) DeleteServiceKeyCallCount() int {
	fake.deleteServiceKeyMutex.RLock()
	defer fake.deleteServiceKeyMutex.RUnlock()
	return len(fake.deleteServiceKeyArgsForCall)
}

func (fake *FakeCloudFoundryClient) DeleteServiceKeyArgsForCall(i int) (cf.ServiceKey, *log.Logger) {
	fake.deleteServiceKeyMutex.RLock()
	defer fake.deleteServiceKeyMutex.RUnlock()
	return fake.deleteServiceKeyArgsForCall[i].serviceKey, fake.deleteServiceKeyArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) DeleteServiceKeyReturns(result1 error) {
	fake.DeleteServiceKeyStub = nil
	fake.deleteServiceKeyReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) DeleteServiceKeyReturnsOnCall(i int, result1 error) {
	fake.DeleteServiceKeyStub = nil
	if fake.deleteServiceKeyReturnsOnCall == nil {
		fake.deleteServiceKeyReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteServiceKeyReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) DeleteServiceInstance(instanceGUID string, logger *log.Logger) error {
	fake.deleteServiceInstanceMutex.Lock()
	ret, specificReturn := fake.deleteServiceInstanceReturnsOnCall[len(fake.deleteServiceInstanceArgsForCall)]
	fake.deleteServiceInstanceArgsForCall = append(fake.deleteServiceInstanceArgsForCall, struct {
		instanceGUID string
		logger       *log.Logger
	}{instanceGUID, logger})
	fake.recordInvocation("DeleteServiceInstance", []interface{}{instanceGUID, logger})
	fake.deleteServiceInstanceMutex.Unlock()
	if fake.DeleteServiceInstanceStub != nil {
		return fake.DeleteServiceInstanceStub(instanceGUID, logger)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.deleteServiceInstanceReturns.result1
}

func (fake *FakeCloudFoundryClient) DeleteServiceInstanceCallCount() int {
	fake.deleteServiceInstanceMutex.RLock()
	defer fake.deleteServiceInstanceMutex.RUnlock()
	return len(fake.deleteServiceInstanceArgsForCall)
}

func (fake *FakeCloudFoundryClient) DeleteServiceInstanceArgsForCall(i int) (string, *log.Logger) {
	fake.deleteServiceInstanceMutex.RLock()
	defer fake.deleteServiceInstanceMutex.RUnlock()
	return fake.deleteServiceInstanceArgsForCall[i].instanceGUID, fake.deleteServiceInstanceArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) DeleteServiceInstanceReturns(result1 error) {
	fake.DeleteServiceInstanceStub = nil
	fake.deleteServiceInstanceReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) DeleteServiceInstanceReturnsOnCall(i int, result1 error) {
	fake.DeleteServiceInstanceStub = nil
	if fake.deleteServiceInstanceReturnsOnCall == nil {
		fake.deleteServiceInstanceReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteServiceInstanceReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getInstancesOfServiceOfferingMutex.RLock()
	defer fake.getInstancesOfServiceOfferingMutex.RUnlock()
	fake.getInstanceMutex.RLock()
	defer fake.getInstanceMutex.RUnlock()
	fake.getBindingsForInstanceMutex.RLock()
	defer fake.getBindingsForInstanceMutex.RUnlock()
	fake.deleteBindingMutex.RLock()
	defer fake.deleteBindingMutex.RUnlock()
	fake.getServiceKeysForInstanceMutex.RLock()
	defer fake.getServiceKeysForInstanceMutex.RUnlock()
	fake.deleteServiceKeyMutex.RLock()
	defer fake.deleteServiceKeyMutex.RUnlock()
	fake.deleteServiceInstanceMutex.RLock()
	defer fake.deleteServiceInstanceMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeCloudFoundryClient) recordInvocation(key string, args []interface{}) {
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

var _ deleter.CloudFoundryClient = new(FakeCloudFoundryClient)
