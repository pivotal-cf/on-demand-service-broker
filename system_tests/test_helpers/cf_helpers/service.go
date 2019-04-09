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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

const (
	CfTimeout     = 2 * time.Minute
	LongCfTimeout = 60 * time.Minute // This is only so long to support a stressed director. It should be combined with smart fail-fast
)

func CreateService(serviceOffering, servicePlan, serviceName, arbitraryParams string) {
	cfArgs := []string{"create-service", serviceOffering, servicePlan, serviceName}
	if arbitraryParams != "" {
		cfArgs = append(cfArgs, "-c", arbitraryParams)
	}

	Eventually(Cf(cfArgs...), CfTimeout).Should(gexec.Exit(0))
	AwaitServiceCreation(serviceName)
}

func CreateServiceWithoutWaiting(serviceOffering, servicePlan, serviceName, arbitraryParams string) {
	cfArgs := []string{"create-service", serviceOffering, servicePlan, serviceName}
	if arbitraryParams != "" {
		cfArgs = append(cfArgs, "-c", arbitraryParams)
	}

	Eventually(Cf(cfArgs...), CfTimeout).Should(gexec.Exit(0))
}

func DeleteServiceWithoutChecking(serviceName string) {
	Eventually(Cf("delete-service", serviceName, "-f"), CfTimeout).Should(gexec.Exit())
	AwaitServiceDeletion(serviceName)
}

func DeleteService(serviceName string) {
	Eventually(Cf("delete-service", serviceName, "-f"), CfTimeout).Should(gexec.Exit(0))
	AwaitServiceDeletion(serviceName)
}

func GetServiceInstanceGUID(serviceName string) string {
	session := Cf("service", serviceName, "--guid")
	Eventually(session, CfTimeout).Should(gexec.Exit(0))
	bytes := session.Out.Contents()
	return strings.TrimSpace(string(bytes))
}

func UpdateServiceToPlan(serviceName, newPlanName string) {
	Eventually(
		Cf("update-service", serviceName, "-p", newPlanName),
		CfTimeout,
	).Should(gexec.Exit(0))
	AwaitServiceUpdate(serviceName)
}

func UpdateServiceWithArbitraryParams(serviceName, arbitraryParams string) {
	Eventually(
		Cf("update-service", serviceName, "-c", arbitraryParams),
		CfTimeout,
	).Should(gexec.Exit(0))
	AwaitServiceUpdate(serviceName)
}

func AwaitInProgressOperations(serviceName string) {
	awaitServiceOperation(
		cfService(serviceName),
		Not(ContainSubstring("in progress")),
		ContainSubstring("in progress"),
		LongCfTimeout,
	)
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
	awaitServiceOperation(cfService(serviceName),
		ContainSubstring(fmt.Sprintf("Service instance %s not found", serviceName)),
		ContainSubstring("failed"),
		LongCfTimeout,
	)
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

func awaitServiceOperation(
	cfCommand func() *gexec.Session,
	successMessageMatcher types.GomegaMatcher,
	failureMessageMatcher types.GomegaMatcher,
	timeout time.Duration,
) {
	Eventually(func() bool {
		session := cfCommand()
		Eventually(session, CfTimeout).Should(gexec.Exit(), "'cf service' command timed out")

		contentsOut := session.Out.Contents()
		contentsErr := session.Err.Contents()
		contentsOut = append(contentsOut, byte(0))
		contents := append(contentsOut, contentsErr...)
		session.Buffer()

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
		return Cf("service", serviceName)
	}
}
