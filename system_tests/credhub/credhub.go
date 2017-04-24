// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package credhub

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

type credhub struct{}

func Login(api, username, password string) (*credhub, error) {
	_, err := execCredhub("login", "-s", api, "-u", username, "-p", password, "--skip-tls-validation")
	if err != nil {
		return nil, fmt.Errorf("unable to login to credhub: %s", err)
	}

	return new(credhub), nil
}

func (c *credhub) Find(pattern string) ([]string, error) {
	results := []string{}

	out, err := execCredhub("find", "-n", pattern)
	if err != nil {
		return results, err
	}

	buf := bufio.NewReader(out)

	// skip column headers
	_, _, err = buf.ReadLine()

	var line []byte
	for err == nil {
		line, _, err = buf.ReadLine()
		if err == nil {
			parts := strings.Split(string(line), " ")
			results = append(results, parts[0])
		}
	}

	if len(results) == 0 {
		return results, fmt.Errorf("Error finding credentials")
	}

	return results, nil
}

func (c *credhub) Get(name string) (string, error) {
	out, err := execCredhub("get", "-n", name, "--output-json")

	if err != nil {
		return "", err
	}

	var cred credential
	if err := json.NewDecoder(out).Decode(&cred); err != nil {
		return "", err
	}

	return cred.Value, nil
}

func execCredhub(args ...string) (io.Reader, error) {
	cmd := exec.Command("credhub", args...)

	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return nil, err
	}

	if err := waitFor(session); err != nil {
		return nil, err
	}

	return bytes.NewReader(session.Out.Contents()), nil
}

func waitFor(s *gexec.Session) error {
	succeeded := eventually(
		func() bool { return s.ExitCode() == 0 },
		3*time.Second,
		10*time.Millisecond,
	)

	if !succeeded {
		return fmt.Errorf("Unexpected error: %s", string(s.Err.Contents()))
	}

	return nil
}

func eventually(fn func() bool, timeout, interval time.Duration) bool {
	expired := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if fn() {
				return true
			}
		case <-expired:
			return false
		}
	}
}

type credential struct {
	Value string `json:"value"`
}
