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
		//TODO set client secret if provided
		instanceClient, err = b.uaaClient.CreateClient(instanceID, getClientSecretFromContext(contextMap), getInstanceNameFromContext(contextMap), getSpaceGUIDFromContext(contextMap))
		if err != nil {
			return nil, err
		}
	}
	return instanceClient, nil
}

func (b *Broker) UpdateServiceInstanceClient(instanceID, dashboardURL string, siClient map[string]string, contextMap map[string]interface{}, logger *log.Logger) error {
	if siClient != nil {
		if b.uaaClient.HasClientDefinition() {
			_, err := b.uaaClient.UpdateClient(instanceID, dashboardURL, getSpaceGUIDFromContext(contextMap))
			return err
		}

		if err := b.uaaClient.DeleteClient(instanceID); err != nil {
			logger.Printf("could not delete the service instance client: %s\n", err.Error())
		}
	}
	return nil
}

func getInstanceNameFromContext(contextMap map[string]interface{}) string {
	var name string
	if rawName, found := contextMap["instance_name"]; found {
		name = rawName.(string)
	}
	return name
}

func getSpaceGUIDFromContext(contextMap map[string]interface{}) string {
	var spaceGUID string
	if rawSpaceGUID, found := contextMap["space_guid"]; found {
		spaceGUID = rawSpaceGUID.(string)
	}
	return spaceGUID
}

func getClientSecretFromContext(contextMap map[string]interface{}) string {
	var clientSecret string
	if rawClientSecret, found := contextMap["client_secret"]; found {
		clientSecret = rawClientSecret.(string)
	}
	return clientSecret
}
