// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package collect_service_metrics_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestOrphanDeployments(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Collect Service Metrics Suite")
}

var binaryPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	binaryPath, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/collect-service-metrics")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binaryPath)
}, func(rawBinaryPath []byte) {
	binaryPath = string(rawBinaryPath)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})
