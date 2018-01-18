// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

type AsyncTaskReporter struct {
	Task     chan int
	Err      chan error
	Finished chan bool
	State    string
}

func NewAsyncTaskReporter() *AsyncTaskReporter {
	return &AsyncTaskReporter{
		Task:     make(chan int, 1),
		Err:      make(chan error, 1),
		Finished: make(chan bool, 1),
	}
}

func (r *AsyncTaskReporter) TaskStarted(taskID int) {
	r.Task <- taskID
}

func (r *AsyncTaskReporter) TaskFinished(taskID int, state string) {
	r.State = state
	r.Finished <- true
}

func (r *AsyncTaskReporter) TaskOutputChunk(taskID int, chunk []byte) {}
