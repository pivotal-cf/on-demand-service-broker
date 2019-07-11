package boshdirector_test

import (
	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pkg/errors"
)

var _ = Describe("GetEvents", func() {
	Context("", func() {
		It("gets the events", func() {
			fakeDirector.EventsReturns([]director.Event{
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "123"}),
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "456"}),
			}, nil)

			filters := director.EventsFilter{Deployment: "deployment-name"}
			events, err := c.GetEvents(filters, logger)

			actualEventsFilter := fakeDirector.EventsArgsForCall(0)

			Expect(actualEventsFilter).To(Equal(filters))
			Expect(err).To(Not(HaveOccurred()))
			Expect(events).To(SatisfyAll(
				ContainElement(boshdirector.BoshEvent{TaskId: "123"}),
				ContainElement(boshdirector.BoshEvent{TaskId: "456"}),
			))
		})

		It("forwards the error when the director fails", func() {
			errorMessage := "failed to get events"
			fakeDirector.EventsReturns([]director.Event{}, errors.New(errorMessage))

			events, err := c.GetEvents(director.EventsFilter{}, logger)

			Expect(err).To(MatchError(ContainSubstring(errorMessage)))
			Expect(events).To(BeEmpty())
		})

		It("forwards the error when it fails to build the director", func() {
			fakeDirectorFactory.NewReturns(fakeDirector, errors.New("fail"))

			events, err := c.GetEvents(director.EventsFilter{Deployment: "deployment-name"}, logger)

			Expect(err).To(Not(MatchError(ContainSubstring("deployment-name"))))
			Expect(fakeDirector.EventsCallCount()).To(BeZero())
			Expect(events).To(BeEmpty())
		})
	})
})
