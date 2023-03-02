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

package delete_all_service_instances_and_deregister_broker_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"io/ioutil"
	"os"
	"testing"
)

var (
	binaryPath, tempDir string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	binary, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/delete-all-service-instances-and-deregister-broker")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binary)
}, func(rawBinary []byte) {
	binaryPath = string(rawBinary)

	var err error
	tempDir, err = ioutil.TempDir("", fmt.Sprintf("broker-integration-tests-%d", GinkgoParallelProcess()))
	Expect(err).ToNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(os.RemoveAll(tempDir)).To(Succeed())
}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestPurgeInstancesAndDeregister(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PurgeInstancesAndDeregister Suite")
}
