package broker

import (
	"log"
)

func (b *Broker) GetServiceInstanceClient(instanceID string, contextMap map[string]interface{}) (map[string]string, error) {
	instanceClient, err := b.uaaClient.GetClient(instanceID)
	if err != nil {
		return nil, err
	}
	if instanceClient == nil {
		instanceClient, err = b.uaaClient.CreateClient(instanceID, getInstanceNameFromContext(contextMap))
		if err != nil {
			return nil, err
		}
	}
	return instanceClient, nil
}

func (b *Broker) UpdateServiceInstanceClient(instanceID string, siClient map[string]string, dashboardURL string, logger *log.Logger) error {
	if siClient != nil {
		if b.uaaClient.HasClientDefinition() {
			_, err := b.uaaClient.UpdateClient(instanceID, dashboardURL)
			return err
		}

		if err := b.uaaClient.DeleteClient(instanceID); err != nil {
			logger.Printf("could not delete the service instance client: %s\n", err.Error())
		}
	}
	return nil
}
