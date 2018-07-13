package boshlinks

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pkg/errors"
)

type DNSRetriever struct {
	httpClient boshdirector.HTTP
}

func NewDNSRetriever(boshHTTP boshdirector.HTTP) boshdirector.DNSRetriever {
	return &DNSRetriever{
		httpClient: boshHTTP,
	}
}

func (d *DNSRetriever) GetLinkAddress(consumerLinkID string, azs []string) (string, error) {
	path := fmt.Sprintf("/link_address?link_id=%s", consumerLinkID)
	for _, az := range azs {
		path += "&azs[]=" + url.PathEscape(az)
	}
	response, err := d.httpClient.RawGet(path)
	if err != nil {
		return "", errors.Wrap(err, "HTTP GET on /link_address endpoint failed: "+response)
	}

	var respObj struct {
		Address string
	}
	err = json.Unmarshal([]byte(response), &respObj)
	if err != nil {
		return "", errors.Wrap(err, "cannot unmarshal link address JSON")
	}

	return respObj.Address, nil
}

func (d *DNSRetriever) LinkProviderID(deploymentName, instanceGroupName, providerName string) (string, error) {
	response, err := d.httpClient.RawGet(fmt.Sprintf("/link_providers?deployment=%s", deploymentName))
	if err != nil {
		return "", errors.Wrap(err, "HTTP GET on /link_providers endpoint failed")
	}

	var providers []struct {
		ID          string
		Name        string
		Shared      bool
		OwnerObject struct {
			Info struct {
				InstanceGroup string `json:"instance_group"`
			} `json:"info"`
		} `json:"owner_object"`
	}

	if err := json.Unmarshal([]byte(response), &providers); err != nil {
		return "", errors.Wrap(err, "cannot unmarshal links provider JSON")
	}

	for _, provider := range providers {
		if provider.Shared && provider.Name == providerName && provider.OwnerObject.Info.InstanceGroup == instanceGroupName {
			return provider.ID, nil
		}
	}

	return "", fmt.Errorf(
		"could not find link provider matching deployment %s, instanceGroupName %s, providerName %s",
		deploymentName, instanceGroupName, providerName)
}

func (d *DNSRetriever) CreateLinkConsumer(providerID string) (string, error) {
	payload := fmt.Sprintf(`
	{
		"link_provider_id":"%s",
		"link_consumer": {
			"owner_object": {
				"name": "external_consumer_id",
				"type": "external"
			}
		}
	}
	`, providerID)

	response, err := d.httpClient.RawPost("/links", payload, "application/json")
	if err != nil {
		return "", errors.Wrap(err, "HTTP POST on /links endpoint failed")
	}

	var respObj struct {
		ID string
	}

	err = json.Unmarshal([]byte(response), &respObj)
	if err != nil {
		return "", errors.Wrap(err, "cannot unmarshal create link consumer response")
	}

	return respObj.ID, nil
}

func (d *DNSRetriever) DeleteLinkConsumer(consumerID string) error {
	response, err := d.httpClient.RawDelete(fmt.Sprintf("/links/%s", consumerID))
	return errors.Wrap(err, fmt.Sprintf("HTTP DELETE on /links/:id endpoint failed: %s", response))
}
