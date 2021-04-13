// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"fmt"
	"log"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

const (
	semiSemverVersionLength = 2
	semverVersionLength     = 3
	stemcellVersionLength   = 4
	uaaTypeString           = "uaa"
)

func (c *Client) GetInfo(logger *log.Logger) (Info, error) {
	var boshInfo Info
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return Info{}, errors.Wrap(err, "Failed to build director")
	}

	directorInfo, err := d.Info()
	if err != nil {
		return Info{}, err
	}

	boshInfo.Version = directorInfo.Version

	if directorInfo.Auth.Type != uaaTypeString {
		return boshInfo, nil
	}

	uaaURL, ok := directorInfo.Auth.Options["url"].(string)
	if ok {
		boshInfo.UserAuthentication = UserAuthentication{
			Options: AuthenticationOptions{
				URL: uaaURL,
			},
		}
	} else {
		return Info{}, errors.New("Cannot retrieve UAA URL from info endpoint")
	}

	return boshInfo, nil
}

func (boshInfo *Info) GetDirectorVersion() (Version, error) {
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
	case semverVersionLength, semiSemverVersionLength:
		versionType = SemverDirectorVersionType
	case stemcellVersionLength:
		versionType = StemcellDirectorVersionType
		versionNumbers = versionNumbers[1:4]
	default:
		return Version{}, unrecognisedBoshDirectorVersionError(rawVersion)
	}

	version, err := semver.ParseTolerant(strings.Join(versionNumbers, "."))
	if err != nil {
		return Version{}, unrecognisedBoshDirectorVersionError(rawVersion)
	}

	return Version{Version: version, Type: versionType}, nil
}

func unrecognisedBoshDirectorVersionError(rawVersion string) error {
	return fmt.Errorf(`unrecognised BOSH Director version: %q`, rawVersion)
}
