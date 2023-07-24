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
	"encoding/json"
	"strings"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func CreateServiceKey(serviceName, serviceKeyName string) {
	cfArgs := []string{"create-service-key", serviceName, serviceKeyName}

	Expect(Cf(cfArgs...)).To(gexec.Exit(0))
}

func GetServiceKey(serviceName, serviceKeyName string) string {
	GinkgoHelper()

	serviceKey := Cf("service-key", serviceName, serviceKeyName)
	Expect(serviceKey).To(gexec.Exit(0))
	serviceKeyContent := string(serviceKey.Buffer().Contents())

	firstBracket := strings.Index(serviceKeyContent, "{")

	var m map[string]any
	Expect(json.Unmarshal([]byte(serviceKeyContent[firstBracket:]), &m)).To(Succeed())

	if nested, ok := m["credentials"].(map[string]any); ok {
		m = nested
	}

	serviceKeyRaw, err := json.Marshal(m)
	Expect(err).NotTo(HaveOccurred())

	return string(serviceKeyRaw)
}

func DeleteServiceKey(serviceName, serviceKeyName string) {
	cfArgs := []string{"delete-service-key", serviceName, serviceKeyName, "-f"}
	Expect(Cf(cfArgs...)).To(gexec.Exit(0))
}

func DeleteServiceKeyWithoutChecking(serviceName, serviceKeyName string) {
	cfArgs := []string{"delete-service-key", serviceName, serviceKeyName, "-f"}
	Expect(Cf(cfArgs...)).To(gexec.Exit())
}

func LooksLikeAServiceKey(key string) {
	var jsonmap map[string]interface{}
	err := json.Unmarshal([]byte(key), &jsonmap)

	Expect(err).NotTo(HaveOccurred())
	Expect(len(jsonmap)).To(BeNumerically(">", 0))
}
