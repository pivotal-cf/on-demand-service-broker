package cf

import (
	"fmt"
	"github.com/pkg/errors"
	"log"
)

func (c Client) GetPlanByServiceInstanceGUID(serviceGUID string, logger *log.Logger) (ServicePlan, error) {
	servicePlanResponse := ServicePlanResponse{}
	err := c.get(fmt.Sprintf("%s%s", c.url, "/v2/service_plans?q=service_instance_guid:"+serviceGUID), &servicePlanResponse, logger)
	if err != nil {
		return ServicePlan{}, errors.Wrap(err, fmt.Sprintf("failed to retrieve plan for service %q", serviceGUID))
	}
	return servicePlanResponse.ServicePlans[0], nil
}

func (c Client) GetServiceInstances(filters GetInstancesFilter, logger *log.Logger) ([]Instance, error) {
	plans, err := c.getPlansForServiceID(filters.ServiceOfferingID, logger)
	if err != nil {
		return nil, err
	}

	query, err := c.createQuery(filters, logger)
	switch err.(type) {
	case ResourceNotFoundError:
		return []Instance{}, nil
	case error:
		return nil, err
	}

	return c.getInstances(plans, query, logger)
}

func (c Client) createQuery(filters GetInstancesFilter, logger *log.Logger) (string, error) {
	var query string
	if filters.OrgName != "" && filters.SpaceName != "" {
		orgResponse, err := c.getOrganization(filters.OrgName, logger)
		if err != nil {
			return "", err
		}

		orgSpacesURL := orgResponse.Resources[0].Entity["spaces_url"].(string)

		spaceResponse, err := c.getSpace(orgSpacesURL, filters.SpaceName, logger)
		if err != nil {
			return "", err
		}

		query = fmt.Sprintf("&q=space_guid:%s", spaceResponse.Resources[0].Metadata["guid"])
	}
	return query, nil
}

func (c Client) getInstances(plans []ServicePlan, query string, logger *log.Logger) ([]Instance, error) {
	instances := []Instance{}
	for _, plan := range plans {
		path := fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?results-per-page=%d%s",
			plan.Metadata.GUID,
			defaultPerPage,
			query,
		)

		for path != "" {
			var serviceInstancesResp serviceInstancesResponse

			instancesURL := fmt.Sprintf("%s%s", c.url, path)

			err := c.get(instancesURL, &serviceInstancesResp, logger)
			if err != nil {
				return nil, err
			}
			for _, instance := range serviceInstancesResp.ServiceInstances {
				instances = append(
					instances,
					Instance{
						GUID:         instance.Metadata.GUID,
						PlanUniqueID: plan.ServicePlanEntity.UniqueID,
					},
				)
			}
			path = serviceInstancesResp.NextPath
		}
	}
	return instances, nil
}
