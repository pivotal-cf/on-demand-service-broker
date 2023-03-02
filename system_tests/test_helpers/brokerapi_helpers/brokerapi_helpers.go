package brokerapi_helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/brokerapi/v9/domain/apiresponses"
)

type BrokerAPIClient struct {
	URI      string
	Username string
	Password string
}

func (b *BrokerAPIClient) Provision(serviceInstanceGUID, serviceID, planID string) apiresponses.ProvisioningResponse {
	url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", b.URI, serviceInstanceGUID)
	provisionDetails := domain.ProvisionDetails{
		ServiceID: serviceID,
		PlanID:    planID,
	}
	resp, bodyContent := b.doRequest(http.MethodPut, url, provisionDetails)
	Expect(resp.StatusCode).To(Equal(http.StatusAccepted), "provision request status unexpected. Response content: "+string(bodyContent))

	provisioningResponse := apiresponses.ProvisioningResponse{}
	Expect(json.Unmarshal(bodyContent, &provisioningResponse)).To(Succeed())
	return provisioningResponse
}

func (b *BrokerAPIClient) PollLastOperation(serviceInstanceGUID, operationData string) {
	sleeps := 0
	lastOperationURL := fmt.Sprintf(
		"http://%s/v2/service_instances/%s/last_operation?operation=%s",
		b.URI,
		serviceInstanceGUID,
		url.QueryEscape(operationData),
	)
	for {
		resp, bodyContent := b.doRequest(http.MethodGet, lastOperationURL, nil)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		lastOperationResponse := apiresponses.LastOperationResponse{}
		err := json.Unmarshal(bodyContent, &lastOperationResponse)
		Expect(err).NotTo(HaveOccurred())

		if lastOperationResponse.State == domain.InProgress {
			time.Sleep(time.Second * 10)
			sleeps += 1
			if sleeps >= 32 {
				Fail("lastOperation timed out")
			}
			continue
		}
		if lastOperationResponse.State == domain.Succeeded {
			return
		}
		if lastOperationResponse.State == domain.Failed {
			Fail("lastOperation returned Failed response")
		}
	}
}

func (b *BrokerAPIClient) Deprovision(serviceInstanceGUID, serviceID, planID string) apiresponses.DeprovisionResponse {
	url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true&service_id=%s&plan_id=%s",
		b.URI, serviceInstanceGUID, serviceID, planID)
	resp, bodyContent := b.doRequest(http.MethodDelete, url, nil)
	Expect(resp.StatusCode).To(Equal(http.StatusAccepted), "delete request status unexpected. Response content: "+string(bodyContent))

	var deprovisionResponse apiresponses.DeprovisionResponse
	Expect(json.Unmarshal(bodyContent, &deprovisionResponse)).To(Succeed())
	return deprovisionResponse
}

func (b *BrokerAPIClient) doRequest(method, url string, reqBody interface{}) (*http.Response, []byte) {
	jsonBody, err := json.Marshal(reqBody)
	Expect(err).NotTo(HaveOccurred())
	body := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequest(method, url, body)
	Expect(err).ToNot(HaveOccurred())
	req.SetBasicAuth(b.Username, b.Password)
	req.Header.Set("X-Broker-API-Version", "2.14")

	req.Close = true

	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	Expect(resp.Body.Close()).To(Succeed())
	return resp, bodyContent
}
