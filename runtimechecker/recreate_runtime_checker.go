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

package runtimechecker

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

type RecreateRuntimeChecker struct {
	BoshInfo boshdirector.Info
}

func (rc *RecreateRuntimeChecker) Check() error {
	directorVersion, _ := rc.BoshInfo.GetDirectorVersion()
	if directorVersion.Version.Major < 266 ||
		(directorVersion.Version.Major == 266 && directorVersion.Version.Minor < 15) ||
		(directorVersion.Version.Major == 267 && directorVersion.Version.Minor < 9) ||
		(directorVersion.Version.Major == 268 &&
			(directorVersion.Version.Minor < 2 || (directorVersion.Version.Minor == 2 && directorVersion.Version.Patch < 2) || directorVersion.Version.Minor == 3)) {
		return errors.New(fmt.Sprintf("Insufficient BOSH director version: %q. The recreate-all errand requires a BOSH director version 268.4.0 or higher, or one of the following patch releases: 266.15.0+, 267.9.0+, 268.2.2+.", directorVersion.Version.String()))
	}

	return nil
}
