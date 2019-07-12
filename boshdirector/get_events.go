package boshdirector

import (
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

type BoshEvent struct {
	TaskId string
}

func (c *Client) GetUpdatesEvents(deploymentName string, logger *log.Logger) ([]BoshEvent, error) {
	return c.getEvents(director.EventsFilter{Deployment: deploymentName, Action: "update", ObjectType: "deployment"}, logger)
}

func (c *Client) GetErrandEvents(deploymentName string, logger *log.Logger) ([]BoshEvent, error) {
	return c.getEvents(director.EventsFilter{Deployment: deploymentName, ObjectType: "errand"}, logger)
}

func (c *Client) getEvents(filter director.EventsFilter, logger *log.Logger) ([]BoshEvent, error) {
	logger.Printf("getting events for %v from bosh", filter)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return []BoshEvent{}, errors.Wrap(err, "Failed to build director")
	}

	events, err := d.Events(filter)
	if err != nil {
		return []BoshEvent{}, errors.Wrap(err, "Failed to get the events using the director")
	}

	var boshEvents []BoshEvent
	for _, event := range events {
		boshEvents = append(boshEvents, BoshEvent{TaskId: event.TaskID()})
	}

	return boshEvents, nil
}
