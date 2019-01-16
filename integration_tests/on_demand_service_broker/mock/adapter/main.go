// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/on_demand_service_broker/mock"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

func main() {
	stderrLogger := log.New(os.Stderr, "[service-adapter] ", log.LstdFlags)
	stderrLogger.Println("processing:")
	for i, arg := range os.Args {
		stderrLogger.Printf("params %d %v\n", i, arg)
	}
	serviceadapter.HandleCommandLineInvocation(os.Args, manifestGenerator(stderrLogger), binder(stderrLogger), dashboardUrlGenerator(stderrLogger))
}

func manifestGenerator(logger *log.Logger) serviceadapter.ManifestGenerator {
	if os.Getenv(mock.NotImplementedManifestGenerator) != "" {
		return nil
	}
	return &Adapter{Logger: logger}
}

func binder(logger *log.Logger) serviceadapter.Binder {
	if os.Getenv(mock.NotImplementedBinder) != "" {
		return nil
	}
	return &Adapter{Logger: logger}
}

func dashboardUrlGenerator(logger *log.Logger) serviceadapter.DashboardUrlGenerator {
	if os.Getenv(mock.NotImplementedDashboardUrl) != "" {
		return nil
	}
	return &Adapter{Logger: logger}
}

type Adapter struct {
	Logger *log.Logger
}

func (a *Adapter) GenerateManifest(params serviceadapter.GenerateManifestParams) (serviceadapter.GenerateManifestOutput, error) {
	errorMessageForOperator := os.Getenv(mock.StderrContentForGenerate)
	if errorMessageForOperator != "" {
		errorMessageForUser := os.Getenv(mock.StdoutContentForGenerate)
		a.Logger.Println(errorMessageForOperator)
		return serviceadapter.GenerateManifestOutput{}, errors.New(errorMessageForUser)
	}

	manifestMapJson := os.Getenv(mock.StdoutContentForGenerate)

	var manifestMap map[string]string
	json.Unmarshal([]byte(manifestMapJson), &manifestMap)
	manifestToReturn, found := manifestMap[params.ServiceDeployment.DeploymentName]
	if !found {
		manifestToReturn = manifestMap[mock.GenerateManifestDefaultKey]
	}

	var manifest bosh.BoshManifest
	if err := yaml.Unmarshal([]byte(manifestToReturn), &manifest); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{}, errors.New("")
	}
	if err := serialiseParameter(mock.InputServiceDeploymentForGenerate, params.ServiceDeployment); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{Manifest: manifest}, errors.New("")
	}
	if err := serialiseParameter(mock.InputPlanForGenerate, params.Plan); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{Manifest: manifest}, errors.New("")
	}
	if err := serialiseParameter(mock.InputRequestParamsForGenerate, params.RequestParams); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{Manifest: manifest}, errors.New("")
	}
	if err := serialiseParameter(mock.InputPreviousManifestForGenerate, params.PreviousManifest); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{Manifest: manifest}, errors.New("")
	}
	if err := serialiseParameter(mock.InputPreviousPlanForGenerate, params.PreviousPlan); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.GenerateManifestOutput{Manifest: manifest}, errors.New("")
	}

	return serviceadapter.GenerateManifestOutput{Manifest: manifest}, nil
}

func (a *Adapter) CreateBinding(params serviceadapter.CreateBindingParams) (serviceadapter.Binding, error) {
	stderrMessage := os.Getenv(mock.StderrContentForBind)
	if stderrMessage != "" {
		a.Logger.Println(stderrMessage)
	}

	switch os.Getenv(mock.ExitCodeForBind) {
	case mock.BindingAlreadyExistsErrorExitCode:
		return serviceadapter.Binding{}, serviceadapter.NewBindingAlreadyExistsError(nil)
	case mock.AppGuidNotProvidedErrorExitCode:
		return serviceadapter.Binding{}, serviceadapter.NewAppGuidNotProvidedError(nil)
	case mock.ErrorExitCode:
		stdoutMessage := os.Getenv(mock.StdoutContentForBind)
		return serviceadapter.Binding{}, errors.New(stdoutMessage)
	}

	credentialsToReturn := os.Getenv(mock.StdoutContentForBind)
	credentials := serviceadapter.Binding{}
	if err := json.Unmarshal([]byte(credentialsToReturn), &credentials); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.Binding{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputIDForBind, params.BindingID); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.Binding{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputBoshVmsForBind, params.DeploymentTopology); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.Binding{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputManifestForBind, params.Manifest); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.Binding{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputRequestParamsForBind, params.RequestParams); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.Binding{}, errors.New("")
	}

	return credentials, nil
}

func (a *Adapter) DeleteBinding(params serviceadapter.DeleteBindingParams) error {
	switch os.Getenv(mock.ExitCodeForUnbind) {
	case mock.BindingNotFoundErrorExitCode:
		return serviceadapter.NewBindingNotFoundError(nil)
	case mock.ErrorExitCode:
		errorMessageForOperator := os.Getenv(mock.StderrContentForUnbind)
		errorMessageForUser := os.Getenv(mock.StdoutContentForUnbind)
		a.Logger.Println(errorMessageForOperator)
		return errors.New(errorMessageForUser)
	}

	if err := serialiseParameter(mock.InputIDForUnBind, params.BindingID); err != nil {
		a.Logger.Println(err.Error())
		return errors.New("")
	}
	if err := serialiseParameter(mock.InputBoshVmsForUnBind, params.DeploymentTopology); err != nil {
		a.Logger.Println(err.Error())
		return errors.New("")
	}
	if err := serialiseParameter(mock.InputManifestForUnBind, params.Manifest); err != nil {
		a.Logger.Println(err.Error())
		return errors.New("")
	}
	if err := serialiseParameter(mock.InputRequestParamsForUnBind, params.RequestParams); err != nil {
		a.Logger.Println(err.Error())
		return errors.New("")
	}

	return nil
}

func (a *Adapter) DashboardUrl(params serviceadapter.DashboardUrlParams) (serviceadapter.DashboardUrl, error) {
	if err := serialiseParameter(mock.InputInstanceIDForGenerateDashboardUrl, params.InstanceID); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.DashboardUrl{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputPlanForGenerateDashboardUrl, params.Plan); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.DashboardUrl{}, errors.New("")
	}

	if err := serialiseParameter(mock.InputManifestForGenerateDashboardUrl, params.Manifest); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.DashboardUrl{}, errors.New("")
	}

	errorMessageForOperator := os.Getenv(mock.StderrContentForDashboardUrl)
	if errorMessageForOperator != "" {
		errorMessageForUser := os.Getenv(mock.StdoutContentForDashboardUrl)
		a.Logger.Println(errorMessageForOperator)
		return serviceadapter.DashboardUrl{}, errors.New(errorMessageForUser)
	}

	dashboardUrl := serviceadapter.DashboardUrl{}
	if err := json.Unmarshal([]byte(os.Getenv(mock.StdoutContentForDashboardUrl)), &dashboardUrl); err != nil {
		a.Logger.Println(err.Error())
		return serviceadapter.DashboardUrl{}, errors.New("")
	}

	return dashboardUrl, nil
}

func serialiseParameter(env string, obj interface{}) error {
	file, err := os.OpenFile(os.Getenv(env), os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(obj); err != nil {
		return fmt.Errorf("error encoding %s: %v", env, err)
	}

	return nil
}
