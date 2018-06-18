// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter

import (
	"log"
	"strings"

	"encoding/json"

	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

type manifest struct {
	Name     string
	Releases []struct {
		Version string
	}
	Stemcells []struct {
		Version string
	}
}

type manifestValidator struct {
	deploymentName string
}

func (c *Client) GenerateManifest(serviceDeployment sdk.ServiceDeployment, plan sdk.Plan, requestParams map[string]interface{}, previousManifest []byte, previousPlan *sdk.Plan, logger *log.Logger) ([]byte, error) {
	serialisedServiceDeployment, err := json.Marshal(serviceDeployment)
	if err != nil {
		return nil, err
	}

	plan.Properties = SanitiseForJSON(plan.Properties)
	serialisedPlan, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}

	serialisedRequestParams, err := json.Marshal(requestParams)
	if err != nil {
		return nil, err
	}
	if previousPlan != nil {
		previousPlan.Properties = SanitiseForJSON(previousPlan.Properties)
	}

	serialisedPreviousPlan, err := json.Marshal(previousPlan)
	if err != nil {
		return nil, err
	}

	var manifest, stdout, stderr []byte
	var exitCode *int
	var jsonErr error

	if c.UsingStdin {
		inputParams := sdk.InputParams{
			GenerateManifest: sdk.GenerateManifestParams{
				ServiceDeployment: string(serialisedServiceDeployment),
				Plan:              string(serialisedPlan),
				RequestParameters: string(serialisedRequestParams),
				PreviousPlan:      string(serialisedPreviousPlan),
				PreviousManifest:  string(previousManifest),
			},
		}

		stdout, stderr, exitCode, err = c.CommandRunner.RunWithInputParams(
			inputParams,
			c.ExternalBinPath, "generate-manifest",
		)
	} else {
		stdout, stderr, exitCode, err = c.CommandRunner.Run(
			c.ExternalBinPath, "generate-manifest",
			string(serialisedServiceDeployment),
			string(serialisedPlan), string(serialisedRequestParams),
			string(previousManifest), string(serialisedPreviousPlan),
		)
	}
	if err != nil {
		return nil, adapterError(c.ExternalBinPath, stdout, stderr, err)
	}

	if err := ErrorForExitCode(*exitCode, string(stdout)); err != nil {
		logger.Printf(adapterFailedMessage(*exitCode, c.ExternalBinPath, stdout, stderr))
		return nil, err
	}

	manifest = stdout
	if c.UsingStdin {
		var manifestOutput sdk.MarshalledGenerateManifest
		jsonErr = json.Unmarshal(stdout, &manifestOutput)
		if jsonErr != nil {
			return nil, adapterError(c.ExternalBinPath, stdout, stderr, jsonErr)
		}
		manifest = []byte(manifestOutput.Manifest)
	}

	logger.Printf("service adapter ran generate-manifest successfully, stderr logs: %s", string(stderr))

	validator := manifestValidator{deploymentName: serviceDeployment.DeploymentName}
	if err := validator.validateManifest(c.ExternalBinPath, manifest, stderr); err != nil {
		return nil, err
	}

	return manifest, nil
}

func (v manifestValidator) validateManifest(adapterPath string, stdout, stderr []byte) error {
	var generatedManifest manifest

	if err := yaml.Unmarshal(stdout, &generatedManifest); err != nil {
		return invalidYAMLError(adapterPath, stderr)
	}

	if generatedManifest.Name != v.deploymentName {
		return incorrectDeploymentNameError(adapterPath, stderr, v.deploymentName, generatedManifest.Name)
	}

	for _, release := range generatedManifest.Releases {
		if strings.HasSuffix(release.Version, "latest") {
			return invalidVersionError(adapterPath, stderr, release.Version)
		}
	}

	for _, stemcell := range generatedManifest.Stemcells {
		if strings.HasSuffix(stemcell.Version, "latest") {
			return invalidVersionError(adapterPath, stderr, stemcell.Version)
		}
	}

	return nil
}
