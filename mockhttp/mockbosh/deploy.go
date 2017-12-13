// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import (
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type deployMock struct {
	expectedManifest []byte
	*mockhttp.Handler
}

func Deploy() *deployMock {
	mock := &deployMock{
		Handler: mockhttp.NewMockedHttpRequest("POST", "/deployments?"),
	}
	mock.WithContentType("text/yaml")
	return mock
}

func (d *deployMock) RedirectsToTask(taskID int) *deployMock {
	d.Handler = d.Handler.RedirectsTo(taskURL(taskID))
	return d
}

func (d *deployMock) WithRawManifest(manifest []byte) *deployMock {
	d.WithBody(string(manifest))
	return d
}

func (d *deployMock) WithManifest(manifest bosh.BoshManifest) *deployMock {
	d.WithBody(toYaml(manifest))
	return d
}

func (d *deployMock) WithAnyContextID() *deployMock {
	d.WithHeaderPresent(BoshContextIDHeader)
	return d
}

func (d *deployMock) WithContextID(value string) *deployMock {
	d.WithHeader(BoshContextIDHeader, value)
	return d
}

func (d *deployMock) WithoutContextID() *deployMock {
	d.WithoutHeader(BoshContextIDHeader)
	return d
}

func (d *deployMock) WaitForChannel(trigger chan bool) *deployMock {
	d.RunsFunction(func() {
		<-trigger
	})
	return d
}

func (d *deployMock) SendToChannel(trigger chan bool) *deployMock {
	d.RunsFunction(func() {
		trigger <- true
	})
	return d
}

func toYaml(obj interface{}) string {
	data, err := yaml.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}
