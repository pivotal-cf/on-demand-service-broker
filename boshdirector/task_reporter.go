// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"encoding/json"

	"log"

	"github.com/cloudfoundry/bosh-cli/director"
)

type BoshTaskOutputReporter struct {
	Output []BoshTaskOutput
	Logger *log.Logger
}

func NewBoshTaskOutputReporter() director.TaskReporter {
	return &BoshTaskOutputReporter{}
}

func (r *BoshTaskOutputReporter) TaskStarted(taskID int) {}

func (r *BoshTaskOutputReporter) TaskFinished(taskID int, state string) {}

func (r *BoshTaskOutputReporter) TaskOutputChunk(taskID int, chunk []byte) {
	output := BoshTaskOutput{}
	err := json.Unmarshal(chunk, &output)
	if err != nil {
		r.Logger.Printf("Unexpected task output: %s\n", string(chunk))
	} else {
		r.Output = append(r.Output, output)
	}
}
