// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	semiSemverVersionLength = 2
	semverVersionLength     = 3
	stemcellVersionLength   = 4
)

func (c *Client) GetDirectorVersion(logger *log.Logger) (Version, error) {
	var boshInfo Info

	err := c.getDataCheckingForErrors(fmt.Sprintf("%s/info", c.url), http.StatusOK, &boshInfo, logger)
	if err != nil {
		return Version{}, err
	}

	version, err := newBoshDirectorVersion(boshInfo.Version)
	if err != nil {
		return Version{}, err
	}

	return version, nil
}

func newBoshDirectorVersion(rawVersion string) (Version, error) {
	trimmedVersion := strings.Fields(rawVersion)
	if len(trimmedVersion) == 0 {
		return Version{}, unrecognisedBoshDirectorVersionError(rawVersion)
	}

	versionPart := trimmedVersion[0]
	versionNumbers := strings.Split(versionPart, ".")

	var versionType VersionType

	switch len(versionNumbers) {
	case semiSemverVersionLength, semverVersionLength:
		versionType = SemverDirectorVersionType
	case stemcellVersionLength:
		versionType = StemcellDirectorVersionType
		versionNumbers = versionNumbers[1:4]
	default:
		return Version{}, unrecognisedBoshDirectorVersionError(rawVersion)
	}

	majorVersion, err := strconv.Atoi(versionNumbers[0])
	if err != nil {
		return Version{}, unrecognisedBoshDirectorVersionError(rawVersion)
	}

	return Version{majorVersion: majorVersion, versionType: versionType}, nil
}

func unrecognisedBoshDirectorVersionError(rawVersion string) error {
	return fmt.Errorf(`unrecognised BOSH Director version: %q`, rawVersion)
}
