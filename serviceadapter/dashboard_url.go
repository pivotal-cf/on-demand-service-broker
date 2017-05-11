// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter

import (
	"encoding/json"
	"log"

	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func (c *Client) GenerateDashboardUrl(instanceID string, plan sdk.Plan, manifest []byte, logger *log.Logger) (string, error) {
	plan.Properties = SanitiseForJSON(plan.Properties)
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}

	stdout, stderr, exitCode, err := c.CommandRunner.Run(c.ExternalBinPath, "dashboard-url", instanceID, string(planJSON), string(manifest))
	if err != nil {
		return "", adapterError(c.ExternalBinPath, stdout, stderr, err)
	}

	if err := ErrorForExitCode(*exitCode, string(stdout)); err != nil {
		logger.Printf(adapterFailedMessage(*exitCode, c.ExternalBinPath, stdout, stderr))
		return "", err
	}

	logger.Printf("service adapter ran dashboard-url successfully, stderr logs: %s", string(stderr))

	dashboardURL := sdk.DashboardUrl{}

	return string(dashboardURL.DashboardUrl), json.Unmarshal(stdout, &dashboardURL)
}
