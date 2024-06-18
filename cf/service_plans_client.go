package cf

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
)

func (c Client) GetPlanByServiceInstanceGUID(serviceGUID string, logger *log.Logger) (ServicePlan, error) {
	servicePlanResponse := ServicePlanResponse{}
	err := c.get(fmt.Sprintf("%s%s", c.url, "/v2/service_plans?q=service_instance_guid:"+serviceGUID), &servicePlanResponse, logger)
	if err != nil {
		return ServicePlan{}, errors.Wrap(err, fmt.Sprintf("failed to retrieve plan for service %q", serviceGUID))
	}
	return servicePlanResponse.ServicePlans[0], nil
}
