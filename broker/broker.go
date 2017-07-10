// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"log"
	"strings"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type Broker struct {
	boshClient     BoshClient
	cfClient       CloudFoundryClient
	adapterClient  ServiceAdapterClient
	deployer       Deployer
	deploymentLock *sync.Mutex

	serviceOffering config.ServiceOffering

	loggerFactory *loggerfactory.LoggerFactory
	featureFlags  FeatureFlags
}

func New(
	boshClient BoshClient,
	cfClient CloudFoundryClient,
	serviceAdapter ServiceAdapterClient,
	deployer Deployer,
	serviceOffering config.ServiceOffering,
	loggerFactory *loggerfactory.LoggerFactory,
	featureFlags FeatureFlags,
) (*Broker, error) {

	b := &Broker{
		boshClient:     boshClient,
		cfClient:       cfClient,
		adapterClient:  serviceAdapter,
		deployer:       deployer,
		deploymentLock: &sync.Mutex{},

		serviceOffering: serviceOffering,

		loggerFactory: loggerFactory,
		featureFlags:  featureFlags,
	}

	if err := b.startupChecks(); err != nil {
		return nil, err
	}

	return b, nil
}

const (
	OperationTypeCreate  = OperationType("create")
	OperationTypeUpdate  = OperationType("update")
	OperationTypeUpgrade = OperationType("upgrade")
	OperationTypeDelete  = OperationType("delete")
	OperationTypeBind    = OperationType("bind")
	OperationTypeUnbind  = OperationType("unbind")
)

type OperationType string

type OperationData struct {
	BoshTaskID           int
	BoshContextID        string `json:",omitempty"`
	OperationType        OperationType
	PlanID               string `json:",omitempty"`
	PostDeployErrandName string `json:",omitempty"`
}

const InstancePrefix = "service-instance_"

func deploymentName(instanceID string) string {
	return InstancePrefix + instanceID
}

func instanceID(deploymentName string) string {
	return strings.TrimPrefix(deploymentName, InstancePrefix)
}

//TODO SF only need to return manifest from Create
//go:generate counterfeiter -o fakes/fake_deployer.go . Deployer
type Deployer interface {
	Create(deploymentName, planID string, requestParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error)
	Update(deploymentName, planID string, applyPendingChanges bool, requestParams map[string]interface{}, previousPlanID *string, boshContextID string, logger *log.Logger) (int, []byte, error)
	Upgrade(deploymentName, planID string, previousPlanID *string, boshContextID string, logger *log.Logger) (int, []byte, error)
}

//go:generate counterfeiter -o fakes/fake_feature_flags.go . FeatureFlags
type FeatureFlags interface {
	CFUserTriggeredUpgrades() bool
}

//go:generate counterfeiter -o fakes/fake_service_adapter_client.go . ServiceAdapterClient
type ServiceAdapterClient interface {
	CreateBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest []byte, requestParams map[string]interface{}, logger *log.Logger) (serviceadapter.Binding, error)
	DeleteBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest []byte, requestParams map[string]interface{}, logger *log.Logger) error
	GenerateDashboardUrl(instanceID string, plan serviceadapter.Plan, manifest []byte, logger *log.Logger) (string, error)
}

//go:generate counterfeiter -o fakes/fake_bosh_client.go . BoshClient
type BoshClient interface {
	GetTask(taskID int, logger *log.Logger) (boshdirector.BoshTask, error)
	GetTasks(deploymentName string, logger *log.Logger) (boshdirector.BoshTasks, error)
	GetNormalisedTasksByContext(deploymentName, contextID string, logger *log.Logger) (boshdirector.BoshTasks, error)
	VMs(deploymentName string, logger *log.Logger) (bosh.BoshVMs, error)
	GetDeployment(name string, logger *log.Logger) ([]byte, bool, error)
	GetDeployments(logger *log.Logger) ([]boshdirector.Deployment, error)
	DeleteDeployment(name, contextID string, logger *log.Logger) (int, error)
	GetDirectorVersion(logger *log.Logger) (boshdirector.Version, error)
	RunErrand(deploymentName, errandName, contextID string, logger *log.Logger) (int, error)
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetAPIVersion(logger *log.Logger) (string, error)
	CountInstancesOfPlan(serviceOfferingID, planID string, logger *log.Logger) (int, error)
	CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[string]int, err error)
	GetInstanceState(serviceInstanceGUID string, logger *log.Logger) (cf.InstanceState, error)
	GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]string, error)
}
