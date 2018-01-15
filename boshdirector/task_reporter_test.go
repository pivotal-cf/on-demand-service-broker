package boshdirector_test

import (
	"fmt"

	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("BoshTaskOutputReporter", func() {
	const taskID = 43
	var (
		taskReporter *boshdirector.BoshTaskOutputReporter
		writer       *gbytes.Buffer
		logger       *log.Logger
	)

	BeforeEach(func() {
		writer = gbytes.NewBuffer()
		logger = log.New(writer, "", 0)
		taskReporter = &boshdirector.BoshTaskOutputReporter{
			Logger: logger,
		}
	})

	It("implements a TaskReporter", func() {
		_, implements := boshdirector.NewBoshTaskOutputReporter().(director.TaskReporter)
		Expect(implements).To(BeTrue())
	})

	It("acts like a reporter", func() {
		By("recording when the task is started")
		taskReporter.TaskStarted(taskID)

		By("recording chunks of output")
		taskReporter.TaskOutputChunk(taskID, toJson(0, "here's the output", ""))
		taskReporter.TaskOutputChunk(taskID, toJson(0, "generate me a sensible struct", "some stderr"))
		taskReporter.TaskOutputChunk(taskID, toJson(0, "i dont care how", ""))
		taskReporter.TaskOutputChunk(taskID, toJson(1, "", "some stderr output"))

		By("recording when the task is finished")
		taskReporter.TaskFinished(taskID, "done")

		By("generating a sensible output")
		sensibleOutput := []boshdirector.BoshTaskOutput{
			{ExitCode: 0, StdOut: "here's the output", StdErr: ""},
			{ExitCode: 0, StdOut: "generate me a sensible struct", StdErr: "some stderr"},
			{ExitCode: 0, StdOut: "i dont care how", StdErr: ""},
			{ExitCode: 1, StdOut: "", StdErr: "some stderr output"},
		}
		Expect(taskReporter.Output).To(Equal(sensibleOutput))
	})

	It("does not fail when json is not valid", func() {
		taskReporter.TaskOutputChunk(taskID, []byte("not json"))

		By("not recording the non-json chunk")
		Expect(taskReporter.Output).To(BeEmpty())

		By("logging the error")
		Expect(writer).To(gbytes.Say("Unexpected task output"))
	})
})

func toJson(exitCode int, stdout, stderr string) []byte {
	return []byte(
		fmt.Sprintf(
			`{
		  "exit_code": %d,
		  "stdout": "%s",
		  "stderr": "%s"
		}`, exitCode, stdout, stderr),
	)
}
