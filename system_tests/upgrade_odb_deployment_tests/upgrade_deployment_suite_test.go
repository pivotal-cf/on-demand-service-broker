// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_deployment_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
)

/*
	This suite expect release urls for previous versions of odb and the adapter.
	Make sure that the tarballs are uploaded on gcloud or elsewhere, and specified in the prepare-env script.

	You can use ../../scripts/build-releases-for-upgrade-test.sh
*/

func TestUpgradeDeploymentTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpgradeDeploymentTests Suite")
}

var _ = BeforeSuite(func() {
	bosh_helpers.UploadRelease(envMustHave("PREVIOUS_ODB_RELEASE_URL"))
	bosh_helpers.UploadRelease(envMustHave("PREVIOUS_ADAPTER_RELEASE_URL"))
})

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).NotTo(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
