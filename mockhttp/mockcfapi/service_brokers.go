package mockcfapi

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
)

const ServiceBrokersResponseTemplate = `{
		"total_results": 1,
		"total_pages": 1,
		"prev_url": null,
		"next_url": null,
		"resources": [
			{
				"metadata": {
					"guid": "%s",
					"url": "/v2/service_brokers/90a413fd-a636-4133-8bfb-a94b07839e96",
					"created_at": "2016-06-08T16:41:22Z",
					"updated_at": "2016-06-08T16:41:22Z"
				},
				"entity": {
					"name": "%s",
					"broker_url": "https://foo.com/url-2",
					"auth_username": "auth_username-2",
					"space_guid": "1d43e64d-ed64-43dd-9046-11f422bd407b"
				}
			}
		]
	}`

type serviceBrokersMock struct {
	*mockhttp.Handler
}

func ListServiceBrokers() *serviceBrokersMock {
	return &serviceBrokersMock{
		mockhttp.NewMockedHttpRequest("GET", "/v2/service_brokers"),
	}
}

func ListServiceBrokersForPage(count int) *serviceBrokersMock {
	return &serviceBrokersMock{
		mockhttp.NewMockedHttpRequest("GET", fmt.Sprintf("/v2/service_brokers?page=%d&results-per-page=100", count)),
	}
}

func DeregisterBroker(serviceBrokerGUID string) *serviceBrokersMock {
	return &serviceBrokersMock{
		mockhttp.NewMockedHttpRequest("DELETE", fmt.Sprintf("/v2/service_brokers/%s", serviceBrokerGUID)),
	}
}

func (m *serviceBrokersMock) RespondsWithBrokers(brokerName, brokerID string) *mockhttp.Handler {
	return m.RespondsOKWith(fmt.Sprintf(ServiceBrokersResponseTemplate, brokerID, brokerName))

}
