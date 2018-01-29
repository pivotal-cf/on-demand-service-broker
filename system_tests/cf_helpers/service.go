// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf_helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

const (
	CfTimeout     = 2 * time.Minute
	LongCfTimeout = time.Minute * 15 // This is only so long to support a stressed director. It should be combined with smart fail-fast
)

func CreateService(serviceOffering, servicePlan, serviceName, arbitraryParams string) {
	cfArgs := []string{"create-service", serviceOffering, servicePlan, serviceName}
	if arbitraryParams != "" {
		cfArgs = append(cfArgs, "-c", arbitraryParams)
	}

	Eventually(cf.Cf(cfArgs...), CfTimeout).Should(gexec.Exit(0))
	AwaitServiceCreation(serviceName)
}

func DeleteService(serviceName string) {
	Eventually(cf.Cf("delete-service", serviceName, "-f"), CfTimeout).Should(gexec.Exit(0))
	AwaitServiceDeletion(serviceName)
}

func AwaitServiceCreation(serviceName string) {
	AwaitServiceCreationWithTimeout(serviceName, LongCfTimeout)
}

func AwaitServiceCreationWithTimeout(serviceName string, timeout time.Duration) {
	awaitServiceOperation(
		cfService(serviceName),
		ContainSubstring("create succeeded"),
		ContainSubstring("failed"),
		timeout,
	)
}

func AwaitServiceDeletion(serviceName string) {
	awaitServicesOperation(serviceName, Not(ContainSubstring(serviceName)))
}

func AwaitServiceUpdate(serviceName string) {
	awaitServiceOperation(
		cfService(serviceName),
		ContainSubstring("update succeeded"),
		ContainSubstring("failed"),
		LongCfTimeout,
	)
}

func AwaitServiceCreationFailure(serviceName string) {
	awaitServiceOperation(
		cfService(serviceName),
		ContainSubstring("create failed"),
		ContainSubstring("succeeded"),
		LongCfTimeout,
	)
}

func AwaitServiceDeletionFailure(serviceName string) {
	awaitServiceOperation(
		cfService(serviceName),
		ContainSubstring("delete failed"),
		Not(ContainSubstring("in progress")),
		LongCfTimeout,
	)
}

func awaitServicesOperation(serviceName string, successMessageMatcher types.GomegaMatcher) {
	cfCommand := func() *gexec.Session {
		return cf.Cf("services")
	}

	Eventually(func() bool {
		session := cfCommand()
		Eventually(session, CfTimeout).Should(gexec.Exit(), "'cf services' command timed out")

		contents := session.Buffer().Contents()

		if strings.Contains(string(contents), "FAILED") &&
			(strings.Contains(string(contents), "Server error, status code:") || strings.Contains(string(contents), "Error reading response")) {
			time.Sleep(time.Second * 5)
			return false
		}

		match, err := successMessageMatcher.Match(contents)
		if err != nil {
			Fail(err.Error())
		}

		if match {
			return true
		}

		lines := strings.Split(string(contents), "\n")
		for _, line := range lines {
			if strings.Contains(line, serviceName) && strings.Contains(line, "failed") {
				Fail(fmt.Sprintf("cf operation on service instance '%s' failed:\n"+string(contents), serviceName))
			}
		}

		time.Sleep(time.Second * 5)
		return false
	}, LongCfTimeout).Should(BeTrue())
}

func awaitServiceOperation(
	cfCommand func() *gexec.Session,
	successMessageMatcher types.GomegaMatcher,
	failureMessageMatcher types.GomegaMatcher,
	timeout time.Duration,
) {
	Eventually(func() bool {
		session := cfCommand()
		Eventually(session, CfTimeout).Should(gexec.Exit())

		contents := session.Buffer().Contents()

		match, err := successMessageMatcher.Match(contents)
		if err != nil {
			Fail(err.Error())
		}

		if match {
			return true
		}

		match, err = failureMessageMatcher.Match(contents)
		if err != nil {
			Fail(err.Error())
		}

		if match {
			Fail("cf operation resulted in unexpected state:\n" + string(contents))
		}

		time.Sleep(time.Second * 5)
		return false
	}, timeout).Should(BeTrue())
}

func cfService(serviceName string) func() *gexec.Session {
	return func() *gexec.Session {
		return cf.Cf("service", serviceName)
	}
}
