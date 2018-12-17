package create_with_maintenance_info_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"net/http"
	"net/url"
	"time"
)

type ProvisionDetailsWithMaintenanceInfo struct {
	ServiceID        string          `json:"service_id"`
	PlanID           string          `json:"plan_id"`
	OrganizationGUID string          `json:"organization_guid"`
	SpaceGUID        string          `json:"space_guid"`
	MaintenanceInfo  json.RawMessage `json:"maintenance_info,omitempty"`
}

var _ = Describe("Creating a service", func() {

	var (
		serviceInstanceGUID string
		serviceId           string
		planId              string
		catalogResponse     brokerapi.CatalogResponse
		provisionDetails    ProvisionDetailsWithMaintenanceInfo
	)

	BeforeEach(func() {
		serviceInstanceGUID = uuid.New()
		catalogResponse = retrieveCatalog()
		serviceId = catalogResponse.Services[0].ID
		planId = catalogResponse.Services[0].Plans[0].ID

		provisionDetails = ProvisionDetailsWithMaintenanceInfo{
			ServiceID:        serviceId,
			PlanID:           planId,
			OrganizationGUID: "orgId",
			SpaceGUID:        "space",
		}
	})

	When("passing maintenance_info which matches ODB's record for the plan", func() {

		AfterEach(func() {
			deleteService(serviceId, planId, serviceInstanceGUID)
		})

		It("accepts the creation request and creates the service", func() {
			maintenanceInfo := catalogResponse.Services[0].Plans[0].MaintenanceInfo
			rawMaintenanceInfo, err := json.Marshal(maintenanceInfo)
			Expect(err).NotTo(HaveOccurred(), "Error marshaling maintenanceInfo")

			provisionDetails.MaintenanceInfo = rawMaintenanceInfo
			provisionJson, err := json.Marshal(provisionDetails)
			Expect(err).NotTo(HaveOccurred())
			body := bytes.NewBuffer(provisionJson)

			url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", brokerInfo.URI, serviceInstanceGUID)
			resp, bodyContent := doRequest(http.MethodPut, url, body)
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			provisioningResponse := brokerapi.ProvisioningResponse{}
			err = json.Unmarshal(bodyContent, &provisioningResponse)
			Expect(err).NotTo(HaveOccurred())

			pollLastOperation(serviceInstanceGUID, provisioningResponse.OperationData)
		})
	})

	When("passing maintenance_info which differs from ODB's record for the plan", func() {

		var (
			actuallyCreatedService bool
			provisionBody          []byte
		)

		AfterEach(func() {
			if actuallyCreatedService {
				provisioningResponse := brokerapi.ProvisioningResponse{}
				err := json.Unmarshal(provisionBody, &provisioningResponse)
				Expect(err).NotTo(HaveOccurred())

				pollLastOperation(serviceInstanceGUID, provisioningResponse.OperationData)
				deleteService(serviceId, planId, serviceInstanceGUID)
			}
		})

		It("fails the creation request and returns MaintenanceInfoConflict", func() {

			maintenanceInfo := catalogResponse.Services[0].Plans[0].MaintenanceInfo
			maintenanceInfo.Public["imposter_property"] = "ho, ho, ho. hmm"
			rawMaintenanceInfo, err := json.Marshal(maintenanceInfo)
			Expect(err).NotTo(HaveOccurred(), "Error marshaling maintenanceInfo")

			provisionDetails.MaintenanceInfo = rawMaintenanceInfo

			provisionJson, err := json.Marshal(provisionDetails)
			Expect(err).NotTo(HaveOccurred())
			body := bytes.NewBuffer(provisionJson)

			url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", brokerInfo.URI, serviceInstanceGUID)
			resp, bodyContent := doRequest(http.MethodPut, url, body)
			if resp.StatusCode == http.StatusAccepted {
				actuallyCreatedService = true
				provisionBody = bodyContent
			}

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			Expect(string(bodyContent)).To(ContainSubstring("maintenanceInfoConflict"))

		})
	})
})

func retrieveCatalog() brokerapi.CatalogResponse {
	resp, bodyContent := doRequest(http.MethodGet, "http://"+brokerInfo.URI+"/v2/catalog", nil)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	var catalogResp = brokerapi.CatalogResponse{}
	err := json.Unmarshal(bodyContent, &catalogResp)
	Expect(err).NotTo(HaveOccurred(), "Error unmarshalling "+string(bodyContent))
	return catalogResp
}

func deleteService(serviceId, planId, serviceInstanceGUID string) {
	resp, bodyContent := doRequest(http.MethodDelete,
		"http://"+brokerInfo.URI+"/v2/service_instances/"+serviceInstanceGUID+"?service_id="+serviceId+"&plan_id="+planId+"&accepts_incomplete=true", nil)
	Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	deprovisionResponse := brokerapi.DeprovisionResponse{}
	err := json.Unmarshal(bodyContent, &deprovisionResponse)
	Expect(err).NotTo(HaveOccurred())
	pollLastOperation(serviceInstanceGUID, deprovisionResponse.OperationData)
}

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
