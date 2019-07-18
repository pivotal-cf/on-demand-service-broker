package boshdirector

import (
	"fmt"
	"log"
	"strconv"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

type BoshEvent struct {
	TaskId int
}

func (c *Client) GetEvents(deploymentName string, action string, logger *log.Logger) ([]BoshEvent, error) {
	filter := director.EventsFilter{Deployment: deploymentName, Action: action, ObjectType: "deployment"}

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
		taskId, err := strconv.Atoi(event.TaskID())
		if err != nil {
			return []BoshEvent{}, errors.New(fmt.Sprintf("could not convert task id %q to int", event.TaskID()))
		}

		boshEvents = append(boshEvents, BoshEvent{TaskId: taskId})
	}

	return boshEvents, nil
}
