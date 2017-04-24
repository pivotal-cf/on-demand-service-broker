// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package adapterclient

import (
	"bytes"
	"os/exec"
	"syscall"
)

//go:generate counterfeiter -o fake_command_runner/fake_command_runner.go . CommandRunner
type CommandRunner interface {
	Run(arg ...string) ([]byte, []byte, *int, error)
}

func NewCommandRunner() CommandRunner {
	return commandRunner{}
}

type commandRunner struct{}

func (c commandRunner) Run(arg ...string) ([]byte, []byte, *int, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(arg[0], arg[1:]...)
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

func intPtr(val int) *int {
	return &val
}
