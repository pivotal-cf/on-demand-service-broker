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

package helpers

import (
	"os/exec"

	"github.com/onsi/gomega/gexec"

	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func StartBinaryWithParams(binaryPath string, params []string) *gexec.Session {
	cmd := exec.Command(binaryPath, params...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func WriteConfig(config []byte, dir string) string {
	configFilePath := filepath.Join(dir, "config.yml")
	Expect(ioutil.WriteFile(configFilePath, config, 0644)).To(Succeed())
	return configFilePath
}
