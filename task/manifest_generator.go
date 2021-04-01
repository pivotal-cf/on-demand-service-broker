// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task

import (
	"log"

	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_service_adapter_client.go . ServiceAdapterClient
type ServiceAdapterClient interface {
	GenerateManifest(
		serviceReleases serviceadapter.ServiceDeployment,
		plan serviceadapter.Plan,
		requestParams map[string]interface{},
		previousManifest []byte,
		previousPlan *serviceadapter.Plan,
		oldSecretsMap map[string]string,
		previousConfigs map[string]string,
		uaaClient map[string]string,
		logger *log.Logger,
	) (serviceadapter.MarshalledGenerateManifest, error)
	GeneratePlanSchema(plan serviceadapter.Plan, logger *log.Logger) (domain.ServiceSchemas, error)
}

type manifestGenerator struct {
	adapterClient    ServiceAdapterClient
	serviceOffering  config.ServiceOffering
	serviceStemcells []serviceadapter.Stemcell
	serviceReleases  serviceadapter.ServiceReleases
}

func NewManifestGenerator(
	serviceAdapter ServiceAdapterClient,
	serviceOffering config.ServiceOffering,
	serviceStemcells []serviceadapter.Stemcell,
	serviceReleases serviceadapter.ServiceReleases,
) manifestGenerator {
	return manifestGenerator{
		adapterClient:    serviceAdapter,
		serviceOffering:  serviceOffering,
		serviceStemcells: serviceStemcells,
		serviceReleases:  serviceReleases,
	}
}

type RawBoshManifest []byte

func (m manifestGenerator) GenerateManifest(
	generateManifestProps GenerateManifestProperties,
	logger *log.Logger,
) (serviceadapter.MarshalledGenerateManifest, error) {

	serviceDeployment := serviceadapter.ServiceDeployment{
		DeploymentName: generateManifestProps.DeploymentName,
		Releases:       m.serviceReleases,
		Stemcells:      m.serviceStemcells,
	}

	plan, previousPlan, err := m.findPlans(generateManifestProps.PlanID, generateManifestProps.PreviousPlanID)
	if err != nil {
		logger.Println(err)
		return serviceadapter.MarshalledGenerateManifest{}, err
	}
	logger.Printf("service adapter will generate manifest for deployment %s\n", generateManifestProps.DeploymentName)

	// TODO: That's a lot of arguments...
	manifest, err := m.adapterClient.GenerateManifest(
		serviceDeployment,
		plan,
		generateManifestProps.RequestParams,
		generateManifestProps.OldManifest,
		previousPlan,
		generateManifestProps.SecretsMap,
		generateManifestProps.PreviousConfigs,
		generateManifestProps.UAAClient,
		logger)
	if err != nil {
		logger.Printf("generate manifest: %v\n", err)
	}
	return manifest, err
}

func (m manifestGenerator) findPlans(planID string, previousPlanID *string) (serviceadapter.Plan, *serviceadapter.Plan, error) {
	plan, err := m.findPlan(planID)
	if err != nil {
		return serviceadapter.Plan{}, nil, err
	}

	if previousPlanID == nil {
		return plan, nil, nil
	}

	previousPlan, err := m.findPreviousPlan(*previousPlanID)
	if err != nil {
		return serviceadapter.Plan{}, nil, err
	}

	return plan, previousPlan, nil
}

func (m manifestGenerator) findPlan(planID string) (serviceadapter.Plan, error) {
	plan, found := m.serviceOffering.FindPlanByID(planID)
	if !found {
		return serviceadapter.Plan{}, broker.PlanNotFoundError{PlanGUID: planID}
	}

	return plan.AdapterPlan(m.serviceOffering.GlobalProperties), nil
}

func (m manifestGenerator) findPreviousPlan(previousPlanID string) (*serviceadapter.Plan, error) {
	previousPlan, found := m.serviceOffering.FindPlanByID(previousPlanID)
	if !found {
		return new(serviceadapter.Plan), broker.PlanNotFoundError{PlanGUID: previousPlanID}
	}

	abridgedPlan := previousPlan.AdapterPlan(m.serviceOffering.GlobalProperties)
	return &abridgedPlan, nil
}
