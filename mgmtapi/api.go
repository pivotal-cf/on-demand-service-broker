// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mgmtapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"context"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/cloud_foundry_client"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

type api struct {
	manageableBroker ManageableBroker
	serviceOffering  config.ServiceOffering
	loggerFactory    *loggerfactory.LoggerFactory
}

//go:generate counterfeiter -o fake_manageable_broker/fake_manageable_broker.go . ManageableBroker
type ManageableBroker interface {
	Instances(logger *log.Logger) ([]string, error)
	OrphanDeployments(logger *log.Logger) ([]string, error)
	Upgrade(ctx context.Context, instanceID string, logger *log.Logger) (broker.OperationData, error)
	CountInstancesOfPlans(logger *log.Logger) (map[string]int, error)
}

type Instance struct {
	InstanceID string `json:"instance_id"`
}

type Deployment struct {
	Name string `json:"deployment_name"`
}

type Metric struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func AttachRoutes(r *mux.Router, manageableBroker ManageableBroker, serviceOffering config.ServiceOffering, loggerFactory *loggerfactory.LoggerFactory) {
	a := &api{manageableBroker: manageableBroker, serviceOffering: serviceOffering, loggerFactory: loggerFactory}
	r.HandleFunc("/mgmt/service_instances", a.listAllInstances).Methods("GET")
	r.HandleFunc("/mgmt/service_instances/{instance_id}", a.upgradeInstance).Methods("PATCH")
	r.HandleFunc("/mgmt/metrics", a.metrics).Methods("GET")
	r.HandleFunc("/mgmt/orphan_deployments", a.listOrphanDeployments).Methods("GET")
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

	instances, err := a.manageableBroker.Instances(logger)
	if err != nil {
		logger.Printf("error occurred querying instances: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	presentableInstances := []Instance{}
	for _, instance := range instances {
		presentableInstances = append(presentableInstances, Instance{InstanceID: instance})
	}

	a.writeJson(w, presentableInstances, logger)
}

func (a *api) upgradeInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID := vars["instance_id"]

	requestID := uuid.New()
	ctx := brokercontext.New(r.Context(), string(broker.OperationTypeUpgrade), requestID, a.serviceOffering.Name, instanceID)

	logger := a.loggerFactory.NewWithContext(ctx)

	operationData, err := a.manageableBroker.Upgrade(ctx, instanceID, logger)

	switch err.(type) {
	case nil:
		w.WriteHeader(http.StatusAccepted)
		a.writeJson(w, operationData, logger)
	case cloud_foundry_client.ResourceNotFoundError:
		w.WriteHeader(http.StatusNotFound)
	case broker.DeploymentNotFoundError:
		w.WriteHeader(http.StatusGone)
	case broker.OperationInProgressError:
		w.WriteHeader(http.StatusConflict)
	case error:
		logger.Printf("error occurred upgrading instance %s: %s", instanceID, err)
		w.WriteHeader(http.StatusInternalServerError)
		a.writeJson(w, brokerapi.ErrorResponse{Description: err.Error()}, logger)
	}
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	logger := a.loggerFactory.NewWithRequestID()

	brokerMetrics := []Metric{}
	instanceCountsByPlan, err := a.manageableBroker.CountInstancesOfPlans(logger)

	if err != nil {
		logger.Printf("error getting instance count for service offering %s: %s", a.serviceOffering.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(instanceCountsByPlan) == 0 {
		logger.Printf("service %s not registered with Cloud Foundry", a.serviceOffering.Name)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	totalInstances := 0

	for planID, instanceCount := range instanceCountsByPlan {
		plan, err := a.getPlan(planID)
		if err != nil {
			logger.Println(err)
			a.writeJson(w, []interface{}{}, logger)
			return
		}

		countMetric := Metric{
			Key:   fmt.Sprintf("/on-demand-broker/%s/%s/total_instances", a.serviceOffering.Name, plan.Name),
			Unit:  "count",
			Value: float64(instanceCount),
		}
		brokerMetrics = append(brokerMetrics, countMetric)

		if plan.Quotas.ServiceInstanceLimit != nil {
			limit := *plan.Quotas.ServiceInstanceLimit
			quotaMetric := Metric{
				Key:   fmt.Sprintf("/on-demand-broker/%s/%s/quota_remaining", a.serviceOffering.Name, plan.Name),
				Unit:  "count",
				Value: float64(limit - instanceCount),
			}
			brokerMetrics = append(brokerMetrics, quotaMetric)
		}

		totalInstances = totalInstances + instanceCount
	}

	totalCountMetric := Metric{
		Key:   fmt.Sprintf("/on-demand-broker/%s/total_instances", a.serviceOffering.Name),
		Unit:  "count",
		Value: float64(totalInstances),
	}
	brokerMetrics = append(brokerMetrics, totalCountMetric)

	if a.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil {
		limit := *a.serviceOffering.GlobalQuotas.ServiceInstanceLimit
		quotaMetric := Metric{
			Key:   fmt.Sprintf("/on-demand-broker/%s/quota_remaining", a.serviceOffering.Name),
			Unit:  "count",
			Value: float64(limit - totalInstances),
		}
		brokerMetrics = append(brokerMetrics, quotaMetric)
	}

	a.writeJson(w, brokerMetrics, logger)
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
