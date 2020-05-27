// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/uaa"
	"log"
	"strings"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"

	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type Broker struct {
	boshClient     BoshClient
	cfClient       CloudFoundryClient
	adapterClient  ServiceAdapterClient
	deployer       Deployer
	secretManager  ManifestSecretManager
	instanceLister service.InstanceLister
	hasher         Hasher
	deploymentLock *sync.Mutex
	bindLock       *sync.Mutex

	serviceOffering           config.ServiceOffering
	ExposeOperationalErrors   bool
	EnablePlanSchemas         bool
	EnableSecureManifests     bool
	SupportBackupAgentBinding bool
	DisableBoshConfigs        bool

	loggerFactory   *loggerfactory.LoggerFactory
	telemetryLogger TelemetryLogger
	catalogLock     sync.Mutex
	cachedCatalog   []domain.Service

	decider Decider

	uaaClient UAAClient
}

func New(
	boshClient BoshClient,
	cfClient CloudFoundryClient,
	serviceOffering config.ServiceOffering,
	brokerConfig config.Broker,
	startupCheckers []StartupChecker,
	serviceAdapter ServiceAdapterClient,
	deployer Deployer,
	manifestSecretManager ManifestSecretManager,
	instanceLister service.InstanceLister,
	hasher Hasher,
	loggerFactory *loggerfactory.LoggerFactory,
	telemetryLogger TelemetryLogger,
	decider Decider) (*Broker, error) {

	b := &Broker{
		boshClient:                boshClient,
		cfClient:                  cfClient,
		adapterClient:             serviceAdapter,
		deployer:                  deployer,
		deploymentLock:            &sync.Mutex{},
		bindLock:                  &sync.Mutex{},
		serviceOffering:           serviceOffering,
		ExposeOperationalErrors:   brokerConfig.ExposeOperationalErrors,
		EnablePlanSchemas:         brokerConfig.EnablePlanSchemas,
		EnableSecureManifests:     brokerConfig.EnableSecureManifests,
		DisableBoshConfigs:        brokerConfig.DisableBoshConfigs,
		SupportBackupAgentBinding: brokerConfig.SupportBackupAgentBinding,
		secretManager:             manifestSecretManager,
		instanceLister:            instanceLister,
		hasher:                    hasher,
		loggerFactory:             loggerFactory,
		telemetryLogger:           telemetryLogger,
		decider:                   decider,
		uaaClient:                 &uaa.NoopClient{},
	}

	var startupCheckErrMessages []string

	for _, checker := range startupCheckers {
		if err := checker.Check(); err != nil {
			startupCheckErrMessages = append(startupCheckErrMessages, err.Error())
		}
	}

	if len(startupCheckErrMessages) > 0 {
		return nil, fmt.Errorf("The following broker startup checks failed: %s", strings.Join(startupCheckErrMessages, "; "))
	}

	return b, nil
}

func (b *Broker) processError(err error, logger *log.Logger) error {
	if err != nil {
		logger.Println(err)
	}
	switch processedError := err.(type) {
	case DisplayableError:
		if b.ExposeOperationalErrors {
			return processedError.ExtendedCFError()
		}
		return processedError.ErrorForCFUser()
	default:
		return processedError
	}
}

func (b *Broker) SetUAAClient(uaaClient UAAClient) {
	b.uaaClient = uaaClient
}

const (
	ComponentName = "on-demand-service-broker"

	OperationTypeCreate      = OperationType("create")
	OperationTypeUpdate      = OperationType("update")
	OperationTypeUpgrade     = OperationType("upgrade")
	OperationTypeRecreate    = OperationType("recreate")
	OperationTypeDelete      = OperationType("delete")
	OperationTypeForceDelete = OperationType("force-delete")
	OperationTypeBind        = OperationType("bind")
	OperationTypeUnbind      = OperationType("unbind")

	MinimumCFVersion                                     = "2.57.0"
	MinimumMajorStemcellDirectorVersionForODB            = 3262
	MinimumMajorSemverDirectorVersionForLifecycleErrands = 261
)

type OperationType string

type OperationData struct {
	BoshTaskID       int
	BoshContextID    string `json:",omitempty"`
	OperationType    OperationType
	PlanID           string           `json:",omitempty"`
	PostDeployErrand PostDeployErrand // DEPRECATED: only needed for compatibility with ODB 0.20.x
	PreDeleteErrand  PreDeleteErrand  // DEPRECATED: only needed for compatibility with ODB 0.20.x
	Errands          []config.Errand  `json:",omitempty"`
}

type Errand struct {
	Name      string   `json:",omitempty"`
	Instances []string `json:",omitempty"`
}

type PostDeployErrand struct {
	Name      string   `json:",omitempty"`
	Instances []string `json:",omitempty"`
}

type PreDeleteErrand struct {
	Name      string   `json:",omitempty"`
	Instances []string `json:",omitempty"`
}

type ManifestSecret struct {
	Name  string
	Path  string
	Value interface{}
}

const InstancePrefix = "service-instance_"

func deploymentName(instanceID string) string {
	return InstancePrefix + instanceID
}

func instanceID(deploymentName string) string {
	return strings.TrimPrefix(deploymentName, InstancePrefix)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_uaa_client.go . UAAClient
type UAAClient interface {
	CreateClient(id, name string) (map[string]string, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_startup_checker.go . StartupChecker
type StartupChecker interface {
	Check() error
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_deployer.go . Deployer
type Deployer interface {
	Create(deploymentName, planID string, requestParams map[string]interface{}, boshContextID string, uaaClient map[string]string, logger *log.Logger) (int, []byte, error)
	Update(deploymentName, planID string, requestParams map[string]interface{}, previousPlanID *string, boshContextID string, secretsMap map[string]string, logger *log.Logger) (int, []byte, error)
	Upgrade(deploymentName string, plan config.Plan, boshContextID string, logger *log.Logger) (int, []byte, error)
	Recreate(deploymentName, planID, boshContextID string, logger *log.Logger) (int, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_service_adapter_client.go . ServiceAdapterClient
type ServiceAdapterClient interface {
	CreateBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest []byte, requestParams map[string]interface{}, secretsMap, dnsAddresses map[string]string, logger *log.Logger) (serviceadapter.Binding, error)
	DeleteBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest []byte, requestParams map[string]interface{}, secretsMap map[string]string, dnsAddresses map[string]string, logger *log.Logger) error
	GenerateDashboardUrl(instanceID string, plan serviceadapter.Plan, manifest []byte, logger *log.Logger) (string, error)
	GeneratePlanSchema(plan serviceadapter.Plan, logger *log.Logger) (domain.ServiceSchemas, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_bosh_client.go . BoshClient
type BoshClient interface {
	GetTask(taskID int, logger *log.Logger) (boshdirector.BoshTask, error)
	GetTasksInProgress(deploymentName string, logger *log.Logger) (boshdirector.BoshTasks, error)
	GetNormalisedTasksByContext(deploymentName, contextID string, logger *log.Logger) (boshdirector.BoshTasks, error)
	VMs(deploymentName string, logger *log.Logger) (bosh.BoshVMs, error)
	GetDeployment(name string, logger *log.Logger) ([]byte, bool, error)
	GetDeployments(logger *log.Logger) ([]boshdirector.Deployment, error)
	DeleteDeployment(name, contextID string, force bool, taskReporter *boshdirector.AsyncTaskReporter, logger *log.Logger) (int, error)
	GetInfo(logger *log.Logger) (boshdirector.Info, error)
	RunErrand(deploymentName, errandName string, errandInstances []string, contextID string, logger *log.Logger, taskReporter *boshdirector.AsyncTaskReporter) (int, error)
	Variables(deploymentName string, logger *log.Logger) ([]boshdirector.Variable, error)
	VerifyAuth(logger *log.Logger) error
	GetDNSAddresses(deploymentName string, requestedDNS []config.BindingDNS) (map[string]string, error)
	Deploy(manifest []byte, contextID string, logger *log.Logger, reporter *boshdirector.AsyncTaskReporter) (int, error)
	Recreate(deploymentName, contextID string, logger *log.Logger, taskReporter *boshdirector.AsyncTaskReporter) (int, error)
	GetConfigs(configName string, logger *log.Logger) ([]boshdirector.BoshConfig, error)
	DeleteConfig(configType, configName string, logger *log.Logger) (bool, error)
	DeleteConfigs(configName string, logger *log.Logger) error
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetAPIVersion(logger *log.Logger) (string, error)
	CountInstancesOfPlan(serviceOfferingID, planID string, logger *log.Logger) (int, error)
	CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[cf.ServicePlan]int, err error)
	GetServiceInstances(filter cf.GetInstancesFilter, logger *log.Logger) ([]cf.Instance, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_telemetry_logger.go . TelemetryLogger
type TelemetryLogger interface {
	LogInstances(instanceLister service.InstanceLister, item string, operation string)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_map_hasher.go . Hasher
type Hasher interface {
	Hash(m map[string]string) string
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_decider.go . Decider

type Decider interface {
	DecideOperation(catalog []domain.Service, details domain.UpdateDetails, logger *log.Logger) (decider.Operation, error)
	CanProvision(catalog []domain.Service, planID string, maintenanceInfo *domain.MaintenanceInfo, logger *log.Logger) error
}
