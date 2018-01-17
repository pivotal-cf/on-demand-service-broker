// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import (
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type deploymentMock struct {
	*mockhttp.Handler
}

func GetDeployment(deploymentName string) *deploymentMock {
	return &deploymentMock{
		Handler: mockhttp.NewMockedHttpRequest("GET", fmt.Sprintf("/deployments/%s", deploymentName)),
	}
}

func (t *deploymentMock) RespondsWithRawManifest(manifest []byte) *mockhttp.Handler {
	data := map[string]string{"manifest": string(manifest)}
	return t.RespondsOKWithJSON(data)
}

func (t *deploymentMock) RespondsWithManifest(manifest bosh.BoshManifest) *mockhttp.Handler {
	data, err := yaml.Marshal(manifest)
	Expect(err).NotTo(HaveOccurred())
	return t.RespondsWithRawManifest(data)
}
