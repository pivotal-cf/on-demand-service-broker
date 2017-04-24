// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package adapterclient

import (
	"encoding/json"
	"log"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

func (a *Adapter) DeleteBinding(bindingID string, deploymentTopology bosh.BoshVMs, manifest []byte, requestParams map[string]interface{}, logger *log.Logger) error {
	serialisedBoshVMs, err := json.Marshal(deploymentTopology)
	if err != nil {
		return err
	}

	serialisedRequestParams, err := json.Marshal(requestParams)
	if err != nil {
		return err
	}

	stdout, stderr, exitCode, err := a.CommandRunner.Run(a.ExternalBinPath, "delete-binding", bindingID, string(serialisedBoshVMs), string(manifest), string(serialisedRequestParams))
	if err != nil {
		return adapterError(a.ExternalBinPath, stdout, stderr, err)
	}

	if err := ErrorForExitCode(*exitCode, string(stdout)); err != nil {
		logger.Printf(adapterFailedMessage(*exitCode, a.ExternalBinPath, stdout, stderr))
		return err
	}

	logger.Printf("service adapter ran delete-binding successfully, stderr logs: %s", string(stderr))
	return nil
}
