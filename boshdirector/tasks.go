// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import "encoding/json"

type BoshTasks []BoshTask

func (t BoshTasks) IncompleteTasks() BoshTasks {
	return t.findTasksInStates(TaskIncomplete)
}

func (t BoshTasks) FailedTasks() BoshTasks {
	return t.findTasksInStates(TaskFailed)
}

func (t BoshTasks) DoneTasks() BoshTasks {
	return t.findTasksInStates(TaskComplete)
}

func (t BoshTasks) ToLog() string {
	output, _ := json.Marshal(t)
	return string(output)
}

func (t BoshTasks) AllTasksAreDone() bool {
	return len(t.DoneTasks()) == len(t)
}

func (t BoshTasks) findTasksInStates(stateType TaskStateType) BoshTasks {
	found := BoshTasks{}
	for _, task := range t {
		if task.StateType() == stateType {
			found = append(found, task)
		}
	}
	return found
}
