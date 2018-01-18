package boshdirector_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"

	. "github.com/onsi/ginkgo"
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
