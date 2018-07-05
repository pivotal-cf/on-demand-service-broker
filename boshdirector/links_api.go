// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
//
// This program and the accompanying materials are made available under the
// terms of the under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
//
// You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package boshdirector

import (
	"encoding/json"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"
)

func (c *Client) GetDNSAddresses(deploymentName string, dnsRequest []config.BindingDNS) (map[string]string, error) {
	addresses := map[string]string{}
	for _, req := range dnsRequest {
		providerId, err := c.LinkProviderID(deploymentName, req.InstanceGroup, req.LinkProvider)
		if err != nil {
			return nil, err
		}
		consumerId, err := c.CreateLinkConsumer(providerId)
		if err != nil {
			return nil, err
		}

		addr, err := c.GetLinkAddress(consumerId)
		if err != nil {
			return nil, err
		}

		c.DeleteLinkConsumer(consumerId)
		addresses[req.Name] = addr
	}
	return addresses, nil
}

func (c *Client) GetLinkAddress(consumerLinkID string) (string, error) {
	response, err := c.boshHTTP.RawGet(fmt.Sprintf("/link_address?link_id=%s", consumerLinkID))
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

func (c *Client) LinkProviderID(deploymentName, instanceGroupName, providerName string) (string, error) {
	response, err := c.boshHTTP.RawGet(fmt.Sprintf("/link_providers?deployment=%s", deploymentName))
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

func (c *Client) CreateLinkConsumer(providerID string) (string, error) {
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

	response, err := c.boshHTTP.RawPost("/links", payload, "application/json")
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

func (c *Client) DeleteLinkConsumer(consumerID string) error {
	response, err := c.boshHTTP.RawDelete(fmt.Sprintf("/links/%s", consumerID))
	return errors.Wrap(err, fmt.Sprintf("HTTP DELETE on /links/:id endpoint failed: %s", response))
}
