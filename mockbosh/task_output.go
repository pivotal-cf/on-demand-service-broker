// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import (
	"bytes"
	"encoding/json"
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type taskOutputMock struct {
	*mockhttp.MockHttp
}

func TaskOutput(taskId int) *taskOutputMock {
	return &taskOutputMock{
		MockHttp: mockhttp.NewMockedHttpRequest("GET", fmt.Sprintf("/tasks/%d/output?type=result", taskId)),
	}
}

func (t *taskOutputMock) RespondsWithVMsOutput(vms []boshclient.BoshVMsOutput) *mockhttp.MockHttp {
	output := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(output)

	for _, line := range vms {
		Expect(encoder.Encode(line)).ToNot(HaveOccurred())
	}

	return t.RespondsWith(string(output.Bytes()))
}

func (t *taskOutputMock) RespondsWithTaskOutput(taskOutput []boshclient.BoshTaskOutput) *mockhttp.MockHttp {
	output := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(output)

	for _, line := range taskOutput {
		Expect(encoder.Encode(line)).ToNot(HaveOccurred())
	}

	return t.RespondsWith(string(output.Bytes()))
}
