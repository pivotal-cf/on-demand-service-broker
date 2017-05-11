// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter

import (
	"errors"
	"fmt"

	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

const SuccessExitCode = 0

type Client struct {
	ExternalBinPath string
	CommandRunner   CommandRunner
}

func SanitiseForJSON(properties sdk.Properties) sdk.Properties {
	propertiesToReturn := sdk.Properties{}

	for k, v := range properties {
		propertiesToReturn[k] = sanitiseValueForJSON(v)
	}

	return propertiesToReturn
}

// structures parsed from YAML contain map[interface{}]interface{}, but JSON marshalling requires map[string]interface{}, so we have to walk the structure and convert any instances of the former to the latter
func sanitiseValueForJSON(obj interface{}) interface{} {
	switch castObj := obj.(type) {

	case map[interface{}]interface{}:
		sanitisedMap := map[string]interface{}{}
		for key, value := range castObj {
			sanitisedMap[key.(string)] = sanitiseValueForJSON(value)
		}
		return sanitisedMap

	case []interface{}:
		var sanitisedSlice []interface{}
		for _, value := range castObj {
			sanitisedSlice = append(sanitisedSlice, sanitiseValueForJSON(value))
		}
		return sanitisedSlice

	default:
		return obj

	}
}

var exitCodeMap = map[int]error{
	SuccessExitCode:                       nil,
	sdk.NotImplementedExitCode:            NotImplementedError{errors.New("command not implemented by service adapter")},
	sdk.AppGuidNotProvidedErrorExitCode:   AppGuidNotProvidedError{errors.New("app GUID not provided")},
	sdk.BindingAlreadyExistsErrorExitCode: BindingAlreadyExistsError{errors.New("binding already exists")},
	sdk.BindingNotFoundErrorExitCode:      BindingNotFoundError{errors.New("binding not found")},
}

func ErrorForExitCode(code int, message string) error {
	if err, found := exitCodeMap[code]; found {
		return err
	}

	return UnknownFailureError{errors.New(message)}
}

type UnknownFailureError struct {
	error
}

type NotImplementedError struct {
	error
}

type AppGuidNotProvidedError struct {
	error
}

type BindingAlreadyExistsError struct {
	error
}

type BindingNotFoundError struct {
	error
}

func NewNotImplementedError(msg string) NotImplementedError {
	return NotImplementedError{errors.New(msg)}
}

func NewUnknownFailureError(msg string) UnknownFailureError {
	return UnknownFailureError{errors.New(msg)}
}

func invalidJSONError(adapterPath string, stdout, stderr []byte, err error) error {
	return fmt.Errorf("external service adapter returned invalid JSON at %s: stdout: '%s', stderr: '%s', JSON error: '%s'", adapterPath, string(stdout), string(stderr), err)
}

func invalidYAMLError(adapterPath string, stderr []byte) error {
	return fmt.Errorf("external service adapter generated manifest that is not valid YAML at %s. stderr: '%s'", adapterPath, string(stderr))
}

func adapterError(adapterPath string, stdout, stderr []byte, err error) error {
	return fmt.Errorf("an error occurred running external service adapter at %s: '%s'. stdout: '%s', stderr: '%s'", adapterPath, err, string(stdout), string(stderr))
}

func incorrectDeploymentNameError(adapterPath string, stderr []byte, expectedName, actualName string) error {
	return fmt.Errorf("external service adapter generated manifest with an incorrect deployment name at %s. expected name: '%s', returned name: '%s', stderr: '%s'", adapterPath, expectedName, actualName, stderr)
}

func invalidVersionError(adapterPath string, stderr []byte, version string) error {
	return fmt.Errorf("external service adapter generated manifest with an incorrect version at %s. expected exact version but returned version: '%s', stderr: '%s'", adapterPath, version, stderr)
}

func adapterFailedMessage(exitCode int, adapterPath string, stdout, stderr []byte) string {
	return fmt.Sprintf("external service adapter exited with %d at %s: stdout: '%s', stderr: '%s'\n", exitCode, adapterPath, stdout, stderr)
}
