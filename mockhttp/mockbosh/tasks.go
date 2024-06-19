// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockbosh

import (
	"fmt"
	"net/url"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

type tasksMock struct {
	*mockhttp.Handler
}

func Tasks(deploymentName string) *tasksMock {
	return &tasksMock{
		Handler: mockhttp.NewMockedHttpRequest("GET", fmt.Sprintf("/tasks?deployment=%s&limit=2147483647&verbose=1", deploymentName)),
	}
}

func TasksInProgress(deploymentName string) *tasksMock {
	return &tasksMock{
		Handler: mockhttp.NewMockedHttpRequest(
			"GET",
			fmt.Sprintf(
				"/tasks?deployment=%s&state=%s&verbose=1",
				deploymentName,
				url.QueryEscape("processing,cancelling,queued"),
			),
		),
	}
}

func (t *tasksMock) RespondsWithNoTasks() *mockhttp.Handler {
	return t.RespondsOKWithJSON([]boshdirector.BoshTask{})
}

func (t *tasksMock) RespondsWithATaskContainingState(state, description string) *mockhttp.Handler {
	return t.RespondsWithATask(boshdirector.BoshTask{
		State:       state,
		Description: description,
	})
}

func (t *tasksMock) RespondsWithATask(task boshdirector.BoshTask) *mockhttp.Handler {
	return t.RespondsOKWithJSON([]boshdirector.BoshTask{task})
}

func taskURL(taskID int) string {
	return fmt.Sprintf("/tasks/%d", taskID)
}
