package boshdirector_test

import (
	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pkg/errors"
)

var _ = Describe("Get Events", func() {
	Context("Get update events", func() {
		It("gets the right events", func() {
			fakeDirector.EventsReturns([]director.Event{
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "123"}),
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "456"}),
			}, nil)

			events, err := c.GetUpdatesEvents("deployment-name", logger)

			actualEventsFilter := fakeDirector.EventsArgsForCall(0)

			Expect(actualEventsFilter).To(Equal(director.EventsFilter{Deployment: "deployment-name", Action: "update", ObjectType: "deployment"}))
			Expect(err).To(Not(HaveOccurred()))
			Expect(events).To(SatisfyAll(
				ContainElement(boshdirector.BoshEvent{TaskId: "123"}),
				ContainElement(boshdirector.BoshEvent{TaskId: "456"}),
			))
		})

		It("forwards the error when it fails to get the events", func() {
			errorMessage := "failed to get events"
			fakeDirector.EventsReturns([]director.Event{}, errors.New(errorMessage))

			events, err := c.GetUpdatesEvents("not-necessary", logger)

			Expect(err).To(MatchError(ContainSubstring("Failed to get the events using the director")))
			Expect(events).To(BeEmpty())
		})

		It("forwards the error when it fails to build the director", func() {
			fakeDirectorFactory.NewReturns(fakeDirector, errors.New("fail"))

			events, err := c.GetUpdatesEvents("deployment-name", logger)

			Expect(err).To(MatchError(ContainSubstring("Failed to build director")))
			Expect(fakeDirector.EventsCallCount()).To(BeZero())
			Expect(events).To(BeEmpty())
		})
	})

	Context("Get errands events", func() {
		It("gets the right errand events", func() {
			fakeDirector.EventsReturns([]director.Event{
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "123"}),
				director.NewEventFromResp(director.Client{}, director.EventResp{TaskID: "456"}),
			}, nil)

			events, err := c.GetErrandEvents("deployment-name", logger)

			actualEventsFilter := fakeDirector.EventsArgsForCall(0)

			Expect(actualEventsFilter).To(Equal(director.EventsFilter{Deployment: "deployment-name", ObjectType: "errand"}))
			Expect(err).To(Not(HaveOccurred()))
			Expect(events).To(SatisfyAll(
				ContainElement(boshdirector.BoshEvent{TaskId: "123"}),
				ContainElement(boshdirector.BoshEvent{TaskId: "456"}),
			))
		})

		It("forwards the error when it fails to get the events", func() {
			errorMessage := "failed to get events"
			fakeDirector.EventsReturns([]director.Event{}, errors.New(errorMessage))

			events, err := c.GetErrandEvents("not-necessary", logger)

			Expect(err).To(MatchError(ContainSubstring("Failed to get the events using the director")))
			Expect(events).To(BeEmpty())
		})

		It("forwards the error when it fails to build the director", func() {
			fakeDirectorFactory.NewReturns(fakeDirector, errors.New("fail"))

			events, err := c.GetErrandEvents("deployment-name", logger)

			Expect(err).To(MatchError(ContainSubstring("Failed to build director")))
			Expect(fakeDirector.EventsCallCount()).To(BeZero())
			Expect(events).To(BeEmpty())
		})
	})
})
