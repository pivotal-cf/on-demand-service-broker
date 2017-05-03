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
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

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
	b                   *broker.Broker
	brokerCreationErr   error
	boshClient          *fakes.FakeBoshClient
	boshDirectorVersion boshclient.BoshDirectorVersion
	cfClient            *fakes.FakeCloudFoundryClient
	credhubClient       *fakes.FakeCredhubClient
	serviceAdapter      *fakes.FakeServiceAdapterClient
	fakeDeployer        *fakes.FakeDeployer
	serviceCatalog      config.ServiceOffering
	logBuffer           *bytes.Buffer
	loggerFactory       *loggerfactory.LoggerFactory
	fakeFeatureFlags    *fakes.FakeFeatureFlags

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
)

var _ = BeforeEach(func() {
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
	}

	postDeployErrandPlan := config.Plan{
		ID: postDeployErrandPlanID,
		LifecycleErrands: &config.LifecycleErrands{
			PostDeploy: "health-check",
		},
		InstanceGroups: []serviceadapter.InstanceGroup{},
	}

	preDeleteErrandPlan := config.Plan{
		ID: preDeleteErrandPlanID,
		LifecycleErrands: &config.LifecycleErrands{
			PreDelete: "cleanup-resources",
		},
		InstanceGroups: []serviceadapter.InstanceGroup{},
	}

	boshClient = new(fakes.FakeBoshClient)
	boshDirectorVersion = boshclient.NewBoshDirectorVersion(boshclient.MinimumMajorSemverDirectorVersionForLifecycleErrands, boshclient.SemverDirectorVersionType)
	boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)
	serviceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeDeployer = new(fakes.FakeDeployer)
	cfClient = new(fakes.FakeCloudFoundryClient)
	cfClient.GetAPIVersionReturns("2.57.0", nil)

	credhubClient = new(fakes.FakeCredhubClient)

	serviceCatalog = config.ServiceOffering{
		ID:               serviceOfferingID,
		Name:             "a-cool-redis-service",
		GlobalProperties: serviceadapter.Properties{"a_global_property": "global_value", "some_other_global_property": "other_global_value"},
		GlobalQuotas: config.Quotas{
			ServiceInstanceLimit: &serviceOfferingServiceInstanceLimit,
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

	fakeFeatureFlags = new(fakes.FakeFeatureFlags)
	fakeFeatureFlags.CFUserTriggeredUpgradesReturns(false)
})

var _ = JustBeforeEach(func() {
	b, brokerCreationErr = broker.New(
		boshClient,
		cfClient,
		credhubClient,
		serviceAdapter,
		fakeDeployer,
		serviceCatalog,
		loggerFactory,
		fakeFeatureFlags,
	)
})

func TestBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Broker Suite")
}

func deploymentName(instanceID string) string {
	return "service-instance_" + instanceID
}
