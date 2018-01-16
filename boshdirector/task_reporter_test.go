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

	It("can unmarshal a complete json string in a single chunk", func() {
		taskReporter.TaskOutputChunk(taskID, toJson(0, "I'm stdout", "and I'm stderr"))
		taskReporter.TaskFinished(taskID, "doesn't matter")
		Expect(taskReporter.Output).To(Equal(boshdirector.BoshTaskOutput{
			ExitCode: 0,
			StdOut:   "I'm stdout",
			StdErr:   "and I'm stderr",
		}))
	})

	It("can unmarshal json sent in more than one chunk", func() {
		chunk1 := []byte(`{"exit_code": 0, "stdout":`)
		chunk2 := []byte(`"this is stdout", "stderr": "this is stderr"}`)

		taskReporter.TaskOutputChunk(taskID, chunk1)
		taskReporter.TaskOutputChunk(taskID, chunk2)
		taskReporter.TaskFinished(taskID, "doesn't matter")
		Expect(taskReporter.Output).To(Equal(
			boshdirector.BoshTaskOutput{ExitCode: 0, StdOut: "this is stdout", StdErr: "this is stderr"},
		))
	})

	It("Output is empty until task is finished", func() {
		taskReporter.TaskOutputChunk(taskID, toJson(0, "I'm stdout", "and I'm stderr"))
		Expect(taskReporter.Output).To(Equal(boshdirector.BoshTaskOutput{}))

		taskReporter.TaskFinished(taskID, "doesn't matter")
		Expect(taskReporter.Output).ToNot(Equal(boshdirector.BoshTaskOutput{}))
	})

	It("does not fail when json is not valid", func() {
		taskReporter.TaskOutputChunk(taskID, []byte("not json"))
		taskReporter.TaskFinished(taskID, "asdf")

		By("not recording the non-json chunk")
		Expect(taskReporter.Output).To(Equal(boshdirector.BoshTaskOutput{}))

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
