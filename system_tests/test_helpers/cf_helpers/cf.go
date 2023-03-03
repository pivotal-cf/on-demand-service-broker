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
	"io"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const RETRY_LIMIT = 3
const COMMAND_TIMEOUT = CfTimeout

func CfWithTimeout(timeout time.Duration, args ...string) *gexec.Session {
	return cfWithTimeoutAndStdin(timeout, nil, args...)
}

func cfWithTimeoutAndStdin(timeout time.Duration, stdin io.Reader, args ...string) *gexec.Session {
	cmd := exec.Command("cf", args...)
	cmd.Stdin = stdin
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	select {
	case <-session.Exited:
	case <-time.After(timeout):
		session.Kill().Wait()
	}

	return session
}

func Cf(args ...string) *gexec.Session {
	return CfWithStdin(nil, args...)
}

func CfWithStdin(stdin io.Reader, args ...string) *gexec.Session {
	var s *gexec.Session
	for i := 0; i < RETRY_LIMIT; i++ {
		s = cfWithTimeoutAndStdin(COMMAND_TIMEOUT, stdin, args...)
		if s.ExitCode() == 0 {
			return s
		}
	}
	return s
}

func CreateSpace(orgName, spaceName string) {
	Expect(Cf("create-space", spaceName, "-o", orgName)).To(gexec.Exit(0), fmt.Sprintf("creating space %q in org %q", orgName, spaceName))
}

func DeleteSpace(orgName, spaceName string) {
	Expect(Cf("delete-space", spaceName, "-o", orgName, "-f")).To(gexec.Exit(0), fmt.Sprintf("deleting space %q in org %q", orgName, spaceName))
}

func CreateOrg(orgName string) {
	Expect(Cf("create-org", orgName)).To(gexec.Exit(0), fmt.Sprintf("creating org %q", orgName))
}

func DeleteOrg(orgName string) {
	Expect(Cf("delete-org", orgName, "-f")).To(gexec.Exit(0), fmt.Sprintf("deleting org %q", orgName))
}

func TargetOrgAndSpace(orgName, spaceName string) {
	Expect(Cf("target", "-o", orgName, "-s", spaceName)).To(gexec.Exit(0), fmt.Sprintf("targeting org %q and space %q", orgName, spaceName))
}
