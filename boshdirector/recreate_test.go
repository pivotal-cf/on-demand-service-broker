package boshdirector_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("recreating a deployment", func() {
	var (
		deploymentName string
		fakeDeployment *fakes.FakeBOSHDeployment
		contextID      string
		taskReporter   *boshdirector.AsyncTaskReporter
		taskID         int
	)

	BeforeEach(func() {
		deploymentName = "jimbob"
		contextID = "some-foo-id"
		taskID = 41
		taskReporter = boshdirector.NewAsyncTaskReporter()
		fakeDeployment = new(fakes.FakeBOSHDeployment)

		fakeDirector.WithContextReturns(fakeDirector)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)

		fakeDeployment.RecreateStub = func(slug director.AllOrInstanceGroupOrInstanceSlug, opts director.RecreateOpts) error {
			taskReporter.TaskStarted(taskID)
			return nil
		}
	})

	It("calls recreate on the real bosh client lib", func() {
		taskID, err := c.Recreate(deploymentName, contextID, logger, taskReporter)
		Expect(err).NotTo(HaveOccurred())

		Expect(taskID).To(Equal(41))
		Expect(fakeDirector.WithContextCallCount()).To(Equal(1))
		actualContext := fakeDirector.WithContextArgsForCall(0)
		Expect(actualContext).To(Equal(contextID))
	})

	It("returns an error when the deployment cannot be found", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("cannot find that deployment"))
		_, err := c.Recreate(deploymentName, contextID, logger, taskReporter)

		Expect(err.Error()).To(ContainSubstring("cannot find that deployment"))
		Expect(err.Error()).To(ContainSubstring("BOSH CLI error"))
	})

	It("returns an error when the recreate cannot be started", func() {
		fakeDeployment.RecreateReturns(errors.New("unable to recreate that deployment"))
		_, err := c.Recreate(deploymentName, contextID, logger, taskReporter)

		Expect(err.Error()).To(ContainSubstring("unable to recreate that deployment"))
		Expect(err.Error()).To(ContainSubstring("Could not recreate deployment"))
	})

	It("does not hang when bosh task is queued for a bit", func() {
		fakeDeployment.RecreateStub = func(slug director.AllOrInstanceGroupOrInstanceSlug, opts director.RecreateOpts) error {
			taskReporter.TaskStarted(taskID)
			time.Sleep(time.Second * 2)
			return nil
		}
		exited := make(chan bool)
		go func() {
			c.Recreate(deploymentName, contextID, logger, taskReporter)
			exited <- true
		}()

		Eventually(exited, time.Millisecond*100).Should(Receive(), "didn't return in time")
	})
})
