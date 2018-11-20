package boshdirector

import (
	"fmt"
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

func (c *Client) Recreate(deploymentName, contextID string, logger *log.Logger, taskReporter *AsyncTaskReporter) (int, error) {
	myDirector, err := c.Director(taskReporter)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to build director")
	}

	myDirector = myDirector.WithContext(contextID)

	deployment, err := myDirector.FindDeployment(deploymentName)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("BOSH CLI error"))
	}

	go func() {
		err = deployment.Recreate(director.AllOrInstanceGroupOrInstanceSlug{}, director.RecreateOpts{Fix: true})
		if err != nil {
			taskReporter.Err <- errors.Wrapf(err, "Could not recreate deployment %s", deploymentName)
		}
	}()

	select {
	case err := <-taskReporter.Err:
		return 0, err
	case id := <-taskReporter.Task:
		return id, nil
	}
}
