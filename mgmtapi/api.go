// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mgmtapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type api struct {
	manageableBroker ManageableBroker
	serviceOffering  config.ServiceOffering
	loggerFactory    *loggerfactory.LoggerFactory
}

//go:generate counterfeiter -o fake_manageable_broker/fake_manageable_broker.go . ManageableBroker
type ManageableBroker interface {
	Instances(filter map[string]string, logger *log.Logger) ([]service.Instance, error)
	OrphanDeployments(logger *log.Logger) ([]string, error)
	Upgrade(ctx context.Context, instanceID string, updateDetails domain.UpdateDetails, logger *log.Logger) (broker.OperationData, error)
	Recreate(ctx context.Context, instanceID string, updateDetails domain.UpdateDetails, logger *log.Logger) (broker.OperationData, error)
	CountInstancesOfPlans(logger *log.Logger) (map[cf.ServicePlan]int, error)
}

type Deployment struct {
	Name string `json:"deployment_name"`
}

func AttachRoutes(r *mux.Router, manageableBroker ManageableBroker, serviceOffering config.ServiceOffering, loggerFactory *loggerfactory.LoggerFactory) {
	a := &api{manageableBroker: manageableBroker, serviceOffering: serviceOffering, loggerFactory: loggerFactory}
	r.HandleFunc("/mgmt/service_instances", a.listAllInstances).Methods("GET")

	r.HandleFunc("/mgmt/service_instances/{instance_id}", a.recreateInstance).
		Methods("PATCH").
		Queries("operation_type", "recreate")

	r.HandleFunc("/mgmt/service_instances/{instance_id}", a.upgradeInstance).
		Methods("PATCH").
		Queries("operation_type", "upgrade")

	r.HandleFunc("/mgmt/service_instances/{instance_id}", badRequestHandler()).
		Methods("PATCH")

	r.HandleFunc("/mgmt/metrics", a.metrics).Methods("GET")
	r.HandleFunc("/mgmt/orphan_deployments", a.listOrphanDeployments).Methods("GET")
}

func badRequestHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (a *api) listOrphanDeployments(w http.ResponseWriter, r *http.Request) {
	logger := a.loggerFactory.NewWithRequestID()

	orphanNames, err := a.manageableBroker.OrphanDeployments(logger)
	if err != nil {
		logger.Printf("error occurred querying orphan deployments: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	orphanDeployments := []Deployment{}
	for _, name := range orphanNames {
		orphanDeployments = append(orphanDeployments, Deployment{Name: name})
	}

	a.writeJson(w, orphanDeployments, logger)
}

func (a *api) listAllInstances(w http.ResponseWriter, r *http.Request) {
	logger := a.loggerFactory.NewWithRequestID()
	var instances []service.Instance
	var err error

	filter := getFilterValues(r)
	instances, err = a.manageableBroker.Instances(filter, logger)
	if err != nil {
		logger.Printf("error occurred querying instances: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	a.writeJson(w, instances, logger)
}

func getFilterValues(r *http.Request) map[string]string {
	values := r.URL.Query()
	filter := map[string]string{}
	for k, v := range values {
		filter[k] = v[0]
	}
	return filter
}

func (a *api) recreateInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instance_id"]

	requestID := uuid.New()
	ctx := brokercontext.New(r.Context(), string(broker.OperationTypeRecreate), requestID, a.serviceOffering.Name, instanceID)

	logger := a.loggerFactory.NewWithContext(ctx)

	var details domain.UpdateDetails
	if err := json.NewDecoder(r.Body).Decode(&details); err != nil {
		logger.Printf("error occurred parsing requests body: %s", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		a.writeJson(w, apiresponses.ErrorResponse{Description: "Error in request body. Invalid JSON"}, logger)
		return
	}

	operationData, err := a.manageableBroker.Recreate(ctx, instanceID, details, logger)

	switch err.(type) {
	case nil:
		w.WriteHeader(http.StatusAccepted)
		a.writeJson(w, operationData, logger)
	case cf.ResourceNotFoundError:
		w.WriteHeader(http.StatusNotFound)
	case broker.DeploymentNotFoundError:
		w.WriteHeader(http.StatusGone)
	case broker.OperationInProgressError:
		w.WriteHeader(http.StatusConflict)
	case error:
		logger.Printf("error occurred recreating instance %s: %s", instanceID, err)
		w.WriteHeader(http.StatusInternalServerError)
		a.writeJson(w, apiresponses.ErrorResponse{Description: err.Error()}, logger)
	}
}

func (a *api) upgradeInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instance_id"]

	requestID := uuid.New()
	ctx := brokercontext.New(r.Context(), string(broker.OperationTypeUpgrade), requestID, a.serviceOffering.Name, instanceID)

	logger := a.loggerFactory.NewWithContext(ctx)

	var details domain.UpdateDetails
	if err := json.NewDecoder(r.Body).Decode(&details); err != nil {
		logger.Printf("error occurred parsing requests body: %s", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		a.writeJson(w, apiresponses.ErrorResponse{Description: "Error in request body. Invalid JSON"}, logger)
		return
	}

	operationData, err := a.manageableBroker.Upgrade(ctx, instanceID, details, logger)

	switch err.(type) {
	case nil:
		w.WriteHeader(http.StatusAccepted)
		a.writeJson(w, operationData, logger)
	case cf.ResourceNotFoundError:
		w.WriteHeader(http.StatusNotFound)
	case broker.DeploymentNotFoundError:
		w.WriteHeader(http.StatusGone)
	case broker.OperationInProgressError:
		w.WriteHeader(http.StatusConflict)
	case error:
		logger.Printf("error occurred upgrading instance %s: %s", instanceID, err)
		w.WriteHeader(http.StatusInternalServerError)
		a.writeJson(w, apiresponses.ErrorResponse{Description: err.Error()}, logger)
	}
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	logger := a.loggerFactory.NewWithRequestID()
	instanceCountsByPlan, err := a.manageableBroker.CountInstancesOfPlans(logger)

	if err != nil {
		logger.Printf("error getting instance count for service offering %s: %s", a.serviceOffering.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(instanceCountsByPlan) == 0 {
		logger.Printf("The %s service broker must be registered with Cloud Foundry before metrics can be collected", a.serviceOffering.Name)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	brokerMetrics := BrokerMetrics{
		serviceOfferingName: a.serviceOffering.Name,
	}

	totalInstances := 0
	globalCostsPerResource := map[string]int{}
	for plan, instanceCount := range instanceCountsByPlan {
		serviceOfferingPlan, err := a.getPlan(plan.ServicePlanEntity.UniqueID)
		if err != nil {
			logger.Println(err)
			a.writeJson(w, []interface{}{}, logger)
			return
		}

		brokerMetrics = brokerMetrics.AddPlanMetric(serviceOfferingPlan.Name, "total_instances", instanceCount)

		if serviceOfferingPlan.Quotas.ServiceInstanceLimit != nil {
			limit := *serviceOfferingPlan.Quotas.ServiceInstanceLimit

			brokerMetrics = brokerMetrics.AddPlanMetric(serviceOfferingPlan.Name, "quota_remaining", limit-instanceCount)
		}

		for resourceType, quota := range serviceOfferingPlan.Quotas.Resources {
			resourceLimit := quota.Limit

			resourceCost := quota.Cost
			usedResources := resourceCost * instanceCount
			if resourceCost != 0 {
				brokerMetrics = brokerMetrics.AddPlanMetric(
					serviceOfferingPlan.Name,
					fmt.Sprintf("%s/used", resourceType),
					usedResources)

				globalCostsPerResource[resourceType] = usedResources + globalCostsPerResource[resourceType]

				if resourceLimit != 0 {
					brokerMetrics = brokerMetrics.AddPlanMetric(
						serviceOfferingPlan.Name,
						fmt.Sprintf("%s/remaining", resourceType),
						resourceLimit-usedResources)
				}
			}
		}

		totalInstances = totalInstances + instanceCount
	}

	brokerMetrics = brokerMetrics.AddGlobalMetric("total_instances", totalInstances)

	if a.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil {
		limit := *a.serviceOffering.GlobalQuotas.ServiceInstanceLimit

		brokerMetrics = brokerMetrics.AddGlobalMetric("quota_remaining", limit-totalInstances)
	}

	for resourceType, quota := range a.serviceOffering.GlobalQuotas.Resources {
		usedResource := globalCostsPerResource[resourceType]

		brokerMetrics = brokerMetrics.AddGlobalMetric(fmt.Sprintf("%s/used", resourceType), usedResource)
		brokerMetrics = brokerMetrics.AddGlobalMetric(fmt.Sprintf("%s/remaining", resourceType), quota.Limit-usedResource)
	}

	a.writeJson(w, brokerMetrics.metrics, logger)
}

func (a *api) writeJson(w io.Writer, obj interface{}, logger *log.Logger) {
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		logger.Printf("error occurred encoding json: %s", err)
	}
}

func (a *api) getPlan(planID string) (config.Plan, error) {
	for _, plan := range a.serviceOffering.Plans {
		if plan.ID == planID {
			return plan, nil
		}
	}
	return config.Plan{}, fmt.Errorf("no plan found with marketplace ID %s", planID)
}
