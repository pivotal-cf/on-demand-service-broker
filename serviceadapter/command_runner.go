// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter

import (
	"bytes"
	"os/exec"
	"syscall"

	"encoding/json"

	"io"

	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func NewCommandRunner() CommandRunner {
	return commandRunner{}
}

type commandRunner struct {
	inputParams sdk.InputParams
}

func (c commandRunner) Run(arg ...string) ([]byte, []byte, *int, error) {
	cmd := exec.Command(arg[0], arg[1:]...)
	return c.run(cmd)
}

func (c commandRunner) RunWithInputParams(inputParams interface{}, arg ...string) ([]byte, []byte, *int, error) {
	cmd := exec.Command(arg[0], arg[1:]...)

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err // not tested
	}

	b := bytes.NewBuffer([]byte{})
	err = json.NewEncoder(b).Encode(inputParams)

	if err != nil {
		return nil, nil, nil, err // not tested
	}

	go func() {
		defer pipe.Close()
		io.WriteString(pipe, b.String())
	}()

	return c.run(cmd)
}

func intPtr(val int) *int {
	return &val
}

func (c commandRunner) run(cmd *exec.Cmd) ([]byte, []byte, *int, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()

	var exitCode *int

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = intPtr(exitErr.Sys().(syscall.WaitStatus).ExitStatus())
			err = nil
		}
	} else {
		exitCode = intPtr(0)
	}

	return stdout, stderr.Bytes(), exitCode, err
}
