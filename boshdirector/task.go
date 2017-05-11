// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import "encoding/json"

type BoshTask struct {
	ID          int
	State       string
	Description string
	Result      string
	ContextID   string `json:"context_id,omitempty"`
}

type TaskStateType int

const (
	TaskQueued     = "queued"
	TaskProcessing = "processing"
	TaskDone       = "done"
	TaskError      = "error"
	TaskCancelled  = "cancelled"
	TaskCancelling = "cancelling"
	TaskTimeout    = "timeout"

	TaskComplete TaskStateType = iota
	TaskIncomplete
	TaskFailed
	TaskUnknown
)

func (t BoshTask) ToLog() string {
	output, _ := json.Marshal(t)
	return string(output)
}

func (t BoshTask) StateType() TaskStateType {
	switch t.State {
	case TaskDone:
		return TaskComplete
	case TaskProcessing, TaskQueued, TaskCancelling:
		return TaskIncomplete
	case TaskCancelled, TaskError, TaskTimeout:
		return TaskFailed
	default:
		return TaskUnknown
	}
}
