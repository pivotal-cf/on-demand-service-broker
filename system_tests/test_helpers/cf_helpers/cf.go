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

package cf_helpers

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const RETRY_LIMIT = 3
const COMMAND_TIMEOUT = CfTimeout

func CfWithTimeout(timeout time.Duration, args ...string) *gexec.Session {
	session := cf.Cf(args...)

	select {
	case <-session.Exited:
	case <-time.After(timeout):
		session.Kill().Wait()
	}
	return session
}

func Cf(args ...string) *gexec.Session {
	var s *gexec.Session
	for i := 0; i < RETRY_LIMIT; i++ {
		s = CfWithTimeout(COMMAND_TIMEOUT, args...)
		if s.ExitCode() == 0 {
			return s
		}
	}
	return s
}

func CreateSpace(orgName, spaceName string) {
	Eventually(Cf("create-space", spaceName, "-o", orgName)).Should(gexec.Exit(0), fmt.Sprintf("creating space %q in org %q", orgName, spaceName))
}

func DeleteSpace(orgName, spaceName string) {
	Eventually(Cf("delete-space", spaceName, "-o", orgName, "-f")).Should(gexec.Exit(0), fmt.Sprintf("deleting space %q in org %q", orgName, spaceName))
}

func CreateOrg(orgName string) {
	Eventually(Cf("create-org", orgName)).Should(gexec.Exit(0), fmt.Sprintf("creating org %q", orgName))
}

func DeleteOrg(orgName string) {
	Eventually(Cf("delete-org", orgName, "-f")).Should(gexec.Exit(0), fmt.Sprintf("deleting org %q", orgName))
}

func TargetOrgAndSpace(orgName, spaceName string) {
	Eventually(Cf("target", "-o", orgName, "-s", spaceName)).Should(gexec.Exit(0), fmt.Sprintf("targeting org %q and space %q", orgName, spaceName))
}
