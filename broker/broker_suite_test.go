// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"bytes"
	"io"
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	servicefakes "github.com/pivotal-cf/on-demand-service-broker/service/fakes"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type quotaCase struct {
	GlobalResourceQuota map[string]config.ResourceQuota
	PlanResourceQuota   map[string]config.ResourceQuota
	GlobalInstanceLimit *int
	PlanInstanceLimit   *int
}

const (
	serviceOfferingID = "service-id"

	existingPlanID                = "some-plan-id"
	existingPlanName              = "I'm a plan"
	existingPlanInstanceGroupName = "instance-group-name"

	secondPlanID                        = "another-plan"
	secondExistingPlanInstanceGroupName = "instance-group-name-the-second"

	postDeployErrandPlanID = "post-deploy-errand-plan-id"
	preDeleteErrandPlanID  = "pre-delete-errand-plan-id"
)

var (
	b                          *broker.Broker
	brokerCreationErr          error
	boshClient                 *fakes.FakeBoshClient
	cfClient                   *fakes.FakeCloudFoundryClient
	serviceAdapter             *fakes.FakeServiceAdapterClient
	fakeDeployer               *fakes.FakeDeployer
	fakeInstanceLister         *servicefakes.FakeInstanceLister
	serviceCatalog             config.ServiceOffering
	logBuffer                  *bytes.Buffer
	loggerFactory              *loggerfactory.LoggerFactory
	brokerConfig               config.Broker
	fakeSecretManager          *fakes.FakeManifestSecretManager
	fakeMapHasher              *fakes.FakeHasher
	fakeMaintenanceInfoChecker *fakes.FakeMaintenanceInfoChecker

	existingPlanServiceInstanceLimit    = 3
	serviceOfferingServiceInstanceLimit = 5

	existingPlan = config.Plan{
		ID:   existingPlanID,
		Name: existingPlanName,
		Update: &serviceadapter.Update{
			Canaries:        1,
			CanaryWatchTime: "100-200",
			UpdateWatchTime: "100-200",
			MaxInFlight:     5,
		},
		Quotas: config.Quotas{
			ServiceInstanceLimit: &existingPlanServiceInstanceLimit,
		},
		Properties: serviceadapter.Properties{
			"super": "no",
		},
		InstanceGroups: []serviceadapter.InstanceGroup{
			{
				Name:               existingPlanInstanceGroupName,
				VMType:             "vm-type",
				PersistentDiskType: "disk-type",
				Instances:          42,
				Networks:           []string{"networks"},
				AZs:                []string{"my-az1", "my-az2"},
			},
			{
				Name:      secondExistingPlanInstanceGroupName,
				VMType:    "vm-type",
				Instances: 55,
				Networks:  []string{"networks2"},
			},
		},
	}

	secondPlan config.Plan

	schemaFixture = domain.ServiceSchemas{
		Instance: domain.ServiceInstanceSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
			Update: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"update_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"update_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
		},
		Binding: domain.ServiceBindingSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"bind_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"bind_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
		},
	}

	schemaWithAdditionalPropertiesAllowedFixture = domain.ServiceSchemas{
		Instance: domain.ServiceInstanceSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]interface{}{
						"auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
			Update: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]interface{}{
						"update_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"update_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
		},
		Binding: domain.ServiceBindingSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"bind_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"bind_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
		},
	}

	schemaWithRequiredPropertiesFixture = domain.ServiceSchemas{
		Instance: domain.ServiceInstanceSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]interface{}{
						"auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
					"required": []string{"auto_create_topics"},
				},
			},
			Update: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]interface{}{
						"update_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"update_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
					"required": []string{"update_auto_create_topics"},
				},
			},
		},
		Binding: domain.ServiceBindingSchema{
			Create: domain.Schema{
				Parameters: map[string]interface{}{
					"$schema":              "http://json-schema.org/draft-04/schema#",
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"bind_auto_create_topics": map[string]interface{}{
							"description": "Auto create topics",
							"type":        "boolean",
						},
						"bind_default_replication_factor": map[string]interface{}{
							"description": "Replication factor",
							"type":        "integer",
						},
					},
				},
			},
		},
	}
)

var _ = BeforeEach(func() {
	brokerConfig = config.Broker{ExposeOperationalErrors: false, EnablePlanSchemas: false}
	secondPlan = config.Plan{
		ID: secondPlanID,
		Properties: serviceadapter.Properties{
			"super":             "yes",
			"a_global_property": "overrides_global_value",
		},
		InstanceGroups: []serviceadapter.InstanceGroup{
			{
				Name:               existingPlanInstanceGroupName,
				VMType:             "vm-type1",
				PersistentDiskType: "disk-type1",
				Instances:          44,
				Networks:           []string{"networks1"},
				AZs:                []string{"my-az4", "my-az5"},
			},
		},
		MaintenanceInfo: &config.MaintenanceInfo{
			Public: map[string]string{
				"plan_specific": "value",
			},
		},
	}

	postDeployErrandPlan := config.Plan{
		ID: postDeployErrandPlanID,
		LifecycleErrands: &serviceadapter.LifecycleErrands{
			PostDeploy: []serviceadapter.Errand{{
				Name:      "health-check",
				Instances: []string{"redis-server/0"},
			}},
		},
		InstanceGroups: []serviceadapter.InstanceGroup{},
	}

	preDeleteErrandPlan := config.Plan{
		ID: preDeleteErrandPlanID,
		LifecycleErrands: &serviceadapter.LifecycleErrands{
			PreDelete: []serviceadapter.Errand{{
				Name:      "cleanup-resources",
				Instances: []string{},
			}},
		},
		InstanceGroups: []serviceadapter.InstanceGroup{},
	}

	boshClient = new(fakes.FakeBoshClient)
	serviceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeDeployer = new(fakes.FakeDeployer)
	fakeSecretManager = new(fakes.FakeManifestSecretManager)
	fakeInstanceLister = new(servicefakes.FakeInstanceLister)
	cfClient = new(fakes.FakeCloudFoundryClient)
	fakeMapHasher = new(fakes.FakeHasher)
	fakeMapHasher.HashStub = ReturnSameValueHasher
	cfClient.GetAPIVersionReturns("2.57.0", nil)
	fakeMaintenanceInfoChecker = new(fakes.FakeMaintenanceInfoChecker)

	serviceCatalog = config.ServiceOffering{
		ID:               serviceOfferingID,
		Name:             "a-cool-redis-service",
		GlobalProperties: serviceadapter.Properties{"a_global_property": "global_value", "some_other_global_property": "other_global_value"},
		GlobalQuotas: config.Quotas{
			ServiceInstanceLimit: &serviceOfferingServiceInstanceLimit,
		},
		MaintenanceInfo: &config.MaintenanceInfo{
			Public: map[string]string{
				"version": "fancy",
			},
			Private: map[string]string{"secret": "secret"},
		},
		Plans: []config.Plan{
			existingPlan,
			secondPlan,
			postDeployErrandPlan,
			preDeleteErrandPlan,
		},
	}

	logBuffer = new(bytes.Buffer)
	loggerFactory = loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "broker-unit-tests", log.LstdFlags)
})

func TestBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Broker Suite")
}

func deploymentName(instanceID string) string {
	return "service-instance_" + instanceID
}

func cfServicePlan(guid, uniqueID, servicePlanUrl, name string) cf.ServicePlan {
	return cf.ServicePlan{
		Metadata: cf.Metadata{
			GUID: guid,
		},
		ServicePlanEntity: cf.ServicePlanEntity{
			UniqueID:            uniqueID,
			ServiceInstancesUrl: servicePlanUrl,
			Name:                name,
		},
	}
}

func createDefaultBroker() *broker.Broker {
	b, brokerCreationErr = createBroker([]broker.StartupChecker{})
	Expect(brokerCreationErr).NotTo(HaveOccurred())
	return b
}

func createBrokerWithAdapter(serviceAdapter *fakes.FakeServiceAdapterClient) *broker.Broker {
	var client broker.CloudFoundryClient = cfClient

	broker, err := broker.New(
		boshClient,
		client,
		serviceCatalog,
		brokerConfig,
		[]broker.StartupChecker{},
		serviceAdapter,
		fakeDeployer,
		fakeSecretManager,
		fakeInstanceLister,
		fakeMapHasher,
		loggerFactory,
		fakeMaintenanceInfoChecker,
	)

	Expect(err).NotTo(HaveOccurred())
	return broker
}

func createBrokerWithServiceCatalog(catalog config.ServiceOffering) *broker.Broker {
	var client broker.CloudFoundryClient = cfClient

	broker, err := broker.New(
		boshClient,
		client,
		catalog,
		brokerConfig,
		[]broker.StartupChecker{},
		serviceAdapter,
		fakeDeployer,
		fakeSecretManager,
		fakeInstanceLister,
		fakeMapHasher,
		loggerFactory,
		fakeMaintenanceInfoChecker,
	)

	Expect(err).NotTo(HaveOccurred())
	return broker
}

func createBroker(startupCheckers []broker.StartupChecker, overrideClient ...broker.CloudFoundryClient) (*broker.Broker, error) {
	var client broker.CloudFoundryClient = cfClient
	if len(overrideClient) > 0 {
		client = overrideClient[0]
	}
	return broker.New(
		boshClient,
		client,
		serviceCatalog,
		brokerConfig,
		startupCheckers,
		serviceAdapter,
		fakeDeployer,
		fakeSecretManager,
		fakeInstanceLister,
		fakeMapHasher,
		loggerFactory,
		fakeMaintenanceInfoChecker,
	)
}

func ReturnSameValueHasher(m map[string]string) string {
	var s string
	for key, value := range m {
		s += key + ":" + value + ";"
	}
	return s
}
