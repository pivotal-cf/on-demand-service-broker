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
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
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
