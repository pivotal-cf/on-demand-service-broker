package broker

import (
	"encoding/json"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"log"
)

func (b *Broker) GetServiceInstanceClient(instanceID string, rawContext json.RawMessage) (map[string]string, error) {
	instanceClient, err := b.uaaClient.GetClient(instanceID)
	if err != nil {
		return nil, err
	}
	if instanceClient == nil {
		instanceClient, err = b.uaaClient.CreateClient(instanceID, getInstanceNameFromContext(rawContext))
		if err != nil {
			return nil, err
		}
	}
	return instanceClient, nil
}

func (b *Broker) UpdateServiceInstanceClient(instanceID string, siClient map[string]string, plan config.Plan, manifest []byte, logger *log.Logger) error {
	if siClient != nil {
		abridgedPlan := plan.AdapterPlan(b.serviceOffering.GlobalProperties)
		dashboardUrl, err := b.adapterClient.GenerateDashboardUrl(instanceID, abridgedPlan, manifest, logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); ok {
				return nil
			}
			return err
		}
		_, err = b.uaaClient.UpdateClient(instanceID, dashboardUrl)
		if err != nil {
			return err
		}
	}
	return nil
}
