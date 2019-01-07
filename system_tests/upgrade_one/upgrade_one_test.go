package upgrade_one_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"

	. "github.com/onsi/gomega"
)

var (
	servicePlan         = "redis-small"
	serviceInstanceName = "small-service-" + uuid.New()[:6]
)

type UpdateBody struct {
	ServiceID       string                    `json:"service_id"`
	MaintenanceInfo brokerapi.MaintenanceInfo `json:"maintenance_info"`
	PlanID          string                    `json:"plan_id"`
	PreviousValues  brokerapi.PreviousValues  `json:"previous_values"`
}

var _ = Describe("Upgrade One Service Instance", func() {
	It("can upgrade a single service instance to the latest version", func() {
		cf.CreateService(brokerInfo.ServiceOffering, servicePlan, serviceInstanceName, "")

		// redeploy the broker, adding a lifecycle errand, and changing the maintenance_info
		brokerInfo = bosh.DeployAndRegisterBroker(brokerInfo.TestSuffix, "add_lifecycle_errand.yml", "update_maintenance_info.yml")

		catalog := getCatalog(brokerInfo)

		updateBody := UpdateBody{
			ServiceID:       catalog.Services[0].ID,
			MaintenanceInfo: *catalog.Services[0].Plans[0].MaintenanceInfo,
			PreviousValues: brokerapi.PreviousValues{
				PlanID: catalog.Services[0].Plans[0].ID,
			},
			PlanID: catalog.Services[0].Plans[0].ID,
		}

		bodyBytes, err := json.Marshal(updateBody)
		Expect(err).NotTo(HaveOccurred())
		body := bytes.NewBuffer(bodyBytes)

		serviceInstanceGUID := cf.GetServiceInstanceGUID(serviceInstanceName)
		url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", brokerInfo.URI, serviceInstanceGUID)
		response, bodyContent := doRequest(http.MethodPatch, url, body)
		Expect(response.StatusCode).To(Equal(http.StatusAccepted))

		updateResponse := brokerapi.UpdateResponse{}
		err = json.Unmarshal(bodyContent, &updateResponse)
		Expect(err).NotTo(HaveOccurred())

		pollLastOperation(serviceInstanceGUID, updateResponse.OperationData)

		tasks := bosh.TasksForDeployment(broker.InstancePrefix + serviceInstanceGUID)
		Expect(tasks).To(HaveLen(3))

		Expect(tasks[0].Description).To(HavePrefix("run errand health-check"), "Post-deploy errand wasn't run")
		Expect(tasks[1].Description).To(ContainSubstring("deploy"), "Expected service instance to have been redeployed")
	})
})

func pollLastOperation(instanceID, operationData string) {
	sleeps := 0
	for {
		resp, bodyContent := doLastOperationRequest(instanceID, operationData)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		lastOperationResponse := brokerapi.LastOperationResponse{}
		err := json.Unmarshal(bodyContent, &lastOperationResponse)
		Expect(err).NotTo(HaveOccurred())

		if lastOperationResponse.State == brokerapi.InProgress {
			time.Sleep(time.Second * 10)
			sleeps += 1
			if sleeps >= 32 {
				Fail("lastOperation timed out")
			}
			continue
		}
		if lastOperationResponse.State == brokerapi.Succeeded {
			return
		}
		if lastOperationResponse.State == brokerapi.Failed {
			Fail("lastOperation returned Failed response")
		}
	}
}

func doLastOperationRequest(instanceID, operationData string) (*http.Response, []byte) {
	lastOperationURL := fmt.Sprintf("http://%s/v2/service_instances/%s/last_operation", brokerInfo.URI, instanceID)

	lastOperationURL = fmt.Sprintf("%s?operation=%s", lastOperationURL, url.QueryEscape(operationData))

	return doRequest(http.MethodGet, lastOperationURL, nil)
}

func getCatalog(brokerInfo bosh.BrokerInfo) brokerapi.CatalogResponse {
	req, err := http.NewRequest(http.MethodGet, "http://"+brokerInfo.URI+"/v2/catalog", nil)
	Expect(err).ToNot(HaveOccurred())

	req.SetBasicAuth("broker", brokerInfo.BrokerPassword)
	req.Header.Set("X-Broker-API-Version", "2.14")
	req.Close = true

	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Body.Close()).To(Succeed())

	catalogResp := brokerapi.CatalogResponse{}
	err = json.Unmarshal(bodyContent, &catalogResp)
	Expect(err).NotTo(HaveOccurred(), "Error unmarshalling "+string(bodyContent))

	return catalogResp
}
