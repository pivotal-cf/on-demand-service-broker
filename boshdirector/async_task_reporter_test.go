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

package boshdirector_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AsyncTaskReporter", func() {
	It("writes to task channel when task is started", func() {
		reporter := boshdirector.NewAsyncTaskReporter()
		reporter.TaskStarted(42)
		Expect(<-reporter.Task).To(Equal(42))
	})

	It("sends true to finished channel when task is finished", func() {
		reporter := boshdirector.NewAsyncTaskReporter()
		reporter.TaskFinished(42, "failed")
		Expect(<-reporter.Finished).To(BeTrue())
	})

	It("saves the task state when task is finished", func() {
		reporter := boshdirector.NewAsyncTaskReporter()
		reporter.TaskFinished(42, "failed")
		Expect(<-reporter.Finished).To(BeTrue())
		Expect(reporter.State).To(Equal("failed"))
	})

})
