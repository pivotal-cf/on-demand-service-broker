// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mock

import (
	"encoding/json"
	"io/ioutil"
	"os"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

const (
	StdoutContentForGenerate     = "TEST_ADAPTER_MANIFEST_TO_RETURN"
	StderrContentForGenerate     = "TEST_ADAPTER_ERROR"
	StdoutContentForBind         = "TEST_ADAPTER_CREDENTIALS"
	StderrContentForBind         = "TEST_ADAPTER_BINDING_ERROR"
	StdoutContentForUnbind       = "TEST_ADAPTER_UNBINDING_STDOUT"
	StderrContentForUnbind       = "TEST_ADAPTER_UNBINDING_STDERR"
	StdoutContentForDashboardUrl = "TEST_ADAPTER_DASHBOARD_URL"
	StderrContentForDashboardUrl = "TEST_ADAPTER_DASHBOARD_URL_ERROR"

	InputServiceDeploymentForGenerate = "TEST_SERVICE_DEPLOYMENT_FILE"
	InputPlanForGenerate              = "TEST_PLAN_FILE"
	InputRequestParamsForGenerate     = "TEST_REQUEST_PARAMS_FILE"
	InputPreviousManifestForGenerate  = "TEST_PREVIOUS_MANIFEST_FILE"
	InputPreviousPlanForGenerate      = "TEST_PREVIOUS_PLAN_FILE"
	InputIDForBind                    = "TEST_BINDING_ID_FILE"
	InputBoshVmsForBind               = "TEST_BOSH_VMS_FILE"
	InputManifestForBind              = "TEST_MANIFEST_FILE"
	InputRequestParamsForBind         = "TEST_BINDING_PARAMS_FILE"
	InputManifestForUnBind            = "TEST_DELETE_BINDING_MANIFEST_FILE"
	InputBoshVmsForUnBind             = "TEST_DELETE_BINDING_BOSH_VMS_FILE"
	InputIDForUnBind                  = "TEST_DELETE_BINDING_ID_FILE"
	InputRequestParamsForUnBind       = "TEST_UNBINDING_PARAMS_FILE"

	InputPlanForGenerateDashboardUrl       = "TEST_PLAN_FILE_DASHBOARD_URL"
	InputManifestForGenerateDashboardUrl   = "TEST_MANIFEST_DASHBOARD_URL"
	InputInstanceIDForGenerateDashboardUrl = "TEST_INSTANCE_ID_DASHBOARD_URL"

	ExitCodeForUnbind = "TEST_UNBINDING_EXIT_CODE"
	ExitCodeForBind   = "TEST_BINDING_EXIT_CODE"

	ErrorExitCode                     = string(rune(serviceadapter.ErrorExitCode))
	BindingNotFoundErrorExitCode      = string(rune(serviceadapter.BindingNotFoundErrorExitCode))
	AppGuidNotProvidedErrorExitCode   = string(rune(serviceadapter.AppGuidNotProvidedErrorExitCode))
	BindingAlreadyExistsErrorExitCode = string(rune(serviceadapter.BindingAlreadyExistsErrorExitCode))

	NotImplementedManifestGenerator = "TEST_MANIFEST_GENERATOR_NOT_IMPLEMENTED"
	NotImplementedBinder            = "TEST_BINDER_NOT_IMPLEMENTED"
	NotImplementedDashboardUrl      = "TEST_DASHBOARD_URL_NOT_IMPEMENTED"

	GenerateManifestDefaultKey = "DEFAULT_KEY_FOR_GENERATE_MANIFEST"
)

var InputFiles = []string{
	InputServiceDeploymentForGenerate,
	InputPlanForGenerate,
	InputPreviousPlanForGenerate,
	InputRequestParamsForGenerate,
	InputPreviousManifestForGenerate,
	InputIDForBind,
	InputBoshVmsForBind,
	InputManifestForBind,
	InputRequestParamsForBind,
	InputManifestForUnBind,
	InputBoshVmsForUnBind,
	InputIDForUnBind,
	InputRequestParamsForUnBind,
	InputPlanForGenerateDashboardUrl,
	InputManifestForGenerateDashboardUrl,
	InputInstanceIDForGenerateDashboardUrl,
}

func decodeTestResponse(env string, obj interface{}) {
	f, err := os.Open(os.Getenv(env))
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	Expect(json.NewDecoder(f).Decode(obj)).To(Succeed())
}

type Adapter struct{}

func (a Adapter) Binder() BinderInterfaceHandler {
	return BinderInterfaceHandler{}
}

func (a Adapter) GenerateManifest() GenerateManifestCommandHandler {
	return GenerateManifestCommandHandler{}
}

func (a Adapter) New() {
	vars := []string{
		AppGuidNotProvidedErrorExitCode,
		StdoutContentForGenerate,
		StderrContentForGenerate,
		StdoutContentForBind,
		StderrContentForBind,
		StdoutContentForUnbind,
		StderrContentForUnbind,
		NotImplementedManifestGenerator,
		NotImplementedBinder,
		NotImplementedDashboardUrl,
		StdoutContentForDashboardUrl,
		StderrContentForDashboardUrl,
		ExitCodeForUnbind,
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}

	for _, f := range InputFiles {
		os.Setenv(f, createTestFile())
	}
}

func (a Adapter) Cleanup() {
	for _, f := range InputFiles {
		path := os.Getenv(f)
		Expect(os.Remove(path)).To(Succeed())
	}
}

func createTestFile() string {
	f, err := ioutil.TempFile("", "integration-tests")
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Close()).To(Succeed())
	return f.Name()
}

type ManifestGeneratorInterfaceHandler struct{}

func (ManifestGeneratorInterfaceHandler) NotImplemented() {
	os.Setenv(NotImplementedManifestGenerator, "true")
}

type BinderInterfaceHandler struct{}

func (BinderInterfaceHandler) NotImplemented() {
	os.Setenv(NotImplementedBinder, "true")
}

func (a Adapter) ManifestGenerator() ManifestGeneratorInterfaceHandler {
	return ManifestGeneratorInterfaceHandler{}
}

type GenerateManifestCommandHandler struct{}

func (g GenerateManifestCommandHandler) ToReturnManifest(manifest string) {
	g.ToReturnManifests(map[string]string{GenerateManifestDefaultKey: manifest})
}

func (GenerateManifestCommandHandler) ToFailWithOperatorError(errString string) {
	os.Unsetenv(StdoutContentForGenerate)
	os.Setenv(StderrContentForGenerate, errString)
}

func (GenerateManifestCommandHandler) ToFailWithCFUserAndOperatorError(stdoutString, stderrString string) {
	os.Setenv(StdoutContentForGenerate, stdoutString)
	os.Setenv(StderrContentForGenerate, stderrString)
}

func (GenerateManifestCommandHandler) ToReturnManifests(manifests map[string]string) {
	str, err := json.Marshal(manifests)
	Expect(err).NotTo(HaveOccurred())
	os.Setenv(StdoutContentForGenerate, string(str))
}

func (GenerateManifestCommandHandler) ReceivedPlan() serviceadapter.Plan {
	var plan serviceadapter.Plan
	decodeTestResponse(InputPlanForGenerate, &plan)
	return plan
}

func (GenerateManifestCommandHandler) ReceivedRequestParams() map[string]interface{} {
	var requestParams map[string]interface{}
	decodeTestResponse(InputRequestParamsForGenerate, &requestParams)
	return requestParams
}

func (GenerateManifestCommandHandler) ReceivedPreviousManifest() *bosh.BoshManifest {
	var previousManifest *bosh.BoshManifest
	decodeTestResponse(InputPreviousManifestForGenerate, &previousManifest)
	return previousManifest
}

func (GenerateManifestCommandHandler) ReceivedDeployment() serviceadapter.ServiceDeployment {
	var serviceDeployment serviceadapter.ServiceDeployment
	decodeTestResponse(InputServiceDeploymentForGenerate, &serviceDeployment)
	return serviceDeployment
}

func (GenerateManifestCommandHandler) ReceivedPreviousPlan() serviceadapter.Plan {
	var plan serviceadapter.Plan
	decodeTestResponse(InputPreviousPlanForGenerate, &plan)
	return plan
}

func (a Adapter) CreateBinding() CreateBindingCommandHandler {
	os.Unsetenv(ExitCodeForBind)
	return CreateBindingCommandHandler{}
}

type CreateBindingCommandHandler struct{}

func (CreateBindingCommandHandler) ReturnsBinding(credentials string) {
	os.Setenv(StdoutContentForBind, credentials)
}

func (CreateBindingCommandHandler) FailsWithOperatorError(errString string) {
	os.Setenv(ExitCodeForBind, ErrorExitCode)
	os.Unsetenv(StdoutContentForBind)
	os.Setenv(StderrContentForBind, errString)
}

func (CreateBindingCommandHandler) FailsWithCFUserAndOperatorError(stdoutString, stderrString string) {
	os.Setenv(ExitCodeForBind, ErrorExitCode)
	os.Setenv(StdoutContentForBind, stdoutString)
	os.Setenv(StderrContentForBind, stderrString)
}

func (CreateBindingCommandHandler) FailsWithBindingAlreadyExistsError() {
	os.Setenv(ExitCodeForBind, BindingAlreadyExistsErrorExitCode)
	os.Unsetenv(StdoutContentForBind)
	os.Unsetenv(StderrContentForBind)
}

func (CreateBindingCommandHandler) FailsWithBindingAlreadyExistsErrorAndStderr(stderrString string) {
	os.Setenv(ExitCodeForBind, BindingAlreadyExistsErrorExitCode)
	os.Unsetenv(StdoutContentForBind)
	os.Setenv(StderrContentForBind, stderrString)
}

func (CreateBindingCommandHandler) FailsWithAppGuidNotProvidedError() {
	os.Setenv(ExitCodeForBind, AppGuidNotProvidedErrorExitCode)
	os.Unsetenv(StdoutContentForBind)
	os.Unsetenv(StderrContentForBind)
}

func (CreateBindingCommandHandler) FailsWithAppGuidNotProvidedErrorAndStderr(stderrString string) {
	os.Setenv(ExitCodeForBind, AppGuidNotProvidedErrorExitCode)
	os.Unsetenv(StdoutContentForBind)
	os.Setenv(StderrContentForBind, stderrString)
}

func (CreateBindingCommandHandler) ReceivedManifest() bosh.BoshManifest {
	var manifest bosh.BoshManifest
	decodeTestResponse(InputManifestForBind, &manifest)
	return manifest
}

func (CreateBindingCommandHandler) ReceivedRequestParameters() map[string]interface{} {
	var params map[string]interface{}
	decodeTestResponse(InputRequestParamsForBind, &params)
	return params
}

func (CreateBindingCommandHandler) ReceivedBoshVms() bosh.BoshVMs {
	var boshVms bosh.BoshVMs
	decodeTestResponse(InputBoshVmsForBind, &boshVms)
	return boshVms
}

func (CreateBindingCommandHandler) ReceivedID() string {
	var bindingID string
	decodeTestResponse(InputIDForBind, &bindingID)
	return bindingID
}

func (a Adapter) DeleteBinding() DeleteBindingCommandHandler {
	os.Unsetenv(ExitCodeForUnbind)
	return DeleteBindingCommandHandler{}
}

type DeleteBindingCommandHandler struct{}

func (DeleteBindingCommandHandler) ReceivedManifest() bosh.BoshManifest {
	var manifest bosh.BoshManifest
	decodeTestResponse(InputManifestForUnBind, &manifest)
	return manifest
}

func (DeleteBindingCommandHandler) ReceivedBoshVms() bosh.BoshVMs {
	var boshVms bosh.BoshVMs
	decodeTestResponse(InputBoshVmsForUnBind, &boshVms)
	return boshVms
}

func (DeleteBindingCommandHandler) ReceivedBindingID() string {
	var bindingID string
	decodeTestResponse(InputIDForUnBind, &bindingID)
	return bindingID
}

func (DeleteBindingCommandHandler) ReceivedRequestParameters() serviceadapter.RequestParameters {
	var params serviceadapter.RequestParameters
	decodeTestResponse(InputRequestParamsForUnBind, &params)
	return params
}

func (DeleteBindingCommandHandler) FailsWithOperatorError(errString string) {
	os.Setenv(ExitCodeForUnbind, ErrorExitCode)
	os.Unsetenv(StdoutContentForUnbind)
	os.Setenv(StderrContentForUnbind, errString)
}

func (DeleteBindingCommandHandler) FailsWithCFUserAndOperatorError(stdoutString, stderrString string) {
	os.Setenv(ExitCodeForUnbind, ErrorExitCode)
	os.Setenv(StdoutContentForUnbind, stdoutString)
	os.Setenv(StderrContentForUnbind, stderrString)
}

func (DeleteBindingCommandHandler) FailsWithBindingNotFoundError() {
	os.Setenv(ExitCodeForUnbind, BindingNotFoundErrorExitCode)
	os.Unsetenv(StdoutContentForUnbind)
	os.Unsetenv(StderrContentForUnbind)
}

func (a Adapter) DashboardUrlGenerator() DashboardUrlInterfaceHandler {
	return DashboardUrlInterfaceHandler{}
}

type DashboardUrlInterfaceHandler struct{}

func (a DashboardUrlInterfaceHandler) NotImplemented() {
	os.Setenv(NotImplementedDashboardUrl, "true")
}

type DashboardUrlCommandHandler struct{}

func (a Adapter) DashboardUrl() DashboardUrlCommandHandler {
	return DashboardUrlCommandHandler{}
}

func (a DashboardUrlCommandHandler) Returns(dashboardUrlJson string) {
	os.Unsetenv(NotImplementedDashboardUrl)
	os.Setenv(StdoutContentForDashboardUrl, dashboardUrlJson)
	os.Unsetenv(StderrContentForDashboardUrl)
}

func (a DashboardUrlCommandHandler) FailsWithOperatorError(stderrString string) {
	os.Unsetenv(NotImplementedDashboardUrl)
	os.Unsetenv(StdoutContentForDashboardUrl)
	os.Setenv(StderrContentForDashboardUrl, stderrString)
}

func (a DashboardUrlCommandHandler) FailsWithCFUserAndOperatorError(stdoutString, stderrString string) {
	os.Unsetenv(NotImplementedDashboardUrl)
	os.Setenv(StdoutContentForDashboardUrl, stdoutString)
	os.Setenv(StderrContentForDashboardUrl, stderrString)
}

func (a DashboardUrlCommandHandler) ReceivedInstanceID() string {
	var instanceId string
	decodeTestResponse(InputInstanceIDForGenerateDashboardUrl, &instanceId)
	return instanceId
}

func (a DashboardUrlCommandHandler) ReceivedManifest() bosh.BoshManifest {
	var val bosh.BoshManifest
	decodeTestResponse(InputManifestForGenerateDashboardUrl, &val)
	return val
}

func (a DashboardUrlCommandHandler) ReceivedPlan() serviceadapter.Plan {
	var val serviceadapter.Plan
	decodeTestResponse(InputPlanForGenerateDashboardUrl, &val)
	return val
}
