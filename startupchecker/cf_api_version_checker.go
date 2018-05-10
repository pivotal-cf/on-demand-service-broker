// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package startupchecker

import (
	"errors"
	"fmt"
	"log"

	"github.com/coreos/go-semver/semver"
)

type CFAPIVersionChecker struct {
	cfClient         CFAPIVersionGetter
	minimumCFVersion string
	logger           *log.Logger
}

func NewCFAPIVersionChecker(cfClient CFAPIVersionGetter, minimumCFVersion string, logger *log.Logger) *CFAPIVersionChecker {
	return &CFAPIVersionChecker{
		cfClient:         cfClient,
		minimumCFVersion: minimumCFVersion,
		logger:           logger,
	}
}

func (c *CFAPIVersionChecker) Check() error {
	rawCFAPIVersion, err := c.cfClient.GetAPIVersion(c.logger)
	if err != nil {
		return errors.New("CF API error: " + err.Error() + ". ODB requires CF v238+.")
	}

	version, err := semver.NewVersion(rawCFAPIVersion)
	if err != nil {
		return fmt.Errorf("Cloud Foundry API version couldn't be parsed. Expected a semver, got: %s.", rawCFAPIVersion)
	}

	if version.LessThan(*semver.New(c.minimumCFVersion)) {
		return errors.New("CF API error: Cloud Foundry API version is insufficient, ODB requires CF v238+.")
	}

	return nil
}

//go:generate counterfeiter -o fakes/fake_cf_api_version_getter.go . CFAPIVersionGetter
type CFAPIVersionGetter interface {
	GetAPIVersion(logger *log.Logger) (string, error)
}
