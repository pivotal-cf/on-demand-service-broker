package with_maintenance_info_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/v11/domain"
	"github.com/pivotal-cf/brokerapi/v11/domain/apiresponses"

	"github.com/pivotal-cf/on-demand-service-broker/broker"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

type ProvisionDetailsWithMaintenanceInfo struct {
	ServiceID        string                 `json:"service_id"`
	PlanID           string                 `json:"plan_id"`
	OrganizationGUID string                 `json:"organization_guid"`
	SpaceGUID        string                 `json:"space_guid"`
	MaintenanceInfo  domain.MaintenanceInfo `json:"maintenance_info"`
}

type UpdateBody struct {
	ServiceID       string                 `json:"service_id"`
	MaintenanceInfo domain.MaintenanceInfo `json:"maintenance_info"`
	PlanID          string                 `json:"plan_id"`
	PreviousValues  domain.PreviousValues  `json:"previous_values"`
}

var _ = Describe("On-demand-broker with maintenance_info", func() {
	var (
		serviceCatalog         apiresponses.CatalogResponse
		serviceInstanceGUID    string
		actuallyCreatedService bool
		provisionBody          []byte
		serviceID, planID      string
	)

	BeforeEach(func() {
		serviceCatalog = retrieveCatalog()
		Expect(len(serviceCatalog.Services[0].Plans)).To(Equal(2))

		serviceID = serviceCatalog.Services[0].ID
		planID = serviceCatalog.Services[0].Plans[0].ID
		serviceInstanceGUID = uuid.New()
	})

	AfterEach(func() {
		if actuallyCreatedService {
			provisioningResponse := apiresponses.ProvisioningResponse{}
			err := json.Unmarshal(provisionBody, &provisioningResponse)
			Expect(err).NotTo(HaveOccurred())

			pollLastOperation(serviceInstanceGUID, provisioningResponse.OperationData)
			deleteService(serviceID, planID, serviceInstanceGUID)
		}
	})

	It("supports the lifecycle of a service instance", func() {
		maintenanceInfoWithPlanSpecifics := serviceCatalog.Services[0].Plans[0].MaintenanceInfo
		maintenanceInfoUsingGlobalDef := serviceCatalog.Services[0].Plans[1].MaintenanceInfo
		By("having maintenance info set in the catalog", func() {
			Expect(maintenanceInfoWithPlanSpecifics).ToNot(BeNil())
			Expect(len(maintenanceInfoWithPlanSpecifics.Public)).ToNot(BeZero(), "maintenance_info.public should not be nil or empty")
			Expect(maintenanceInfoWithPlanSpecifics.Private).ToNot(BeZero(), "maintenance_info.private should not be nil or empty")

			Expect(maintenanceInfoUsingGlobalDef.Version).To(Equal("7.8.8+test.123"), "maintenance_info.version does not match")
			Expect(maintenanceInfoUsingGlobalDef.Description).To(Equal("bumped stemcell and rotated cert"), "maintenance_info.description does not match")

			Expect(maintenanceInfoWithPlanSpecifics.Version).To(Equal("8.0.0+git"), "maintenance_info.version does not match")
			Expect(maintenanceInfoWithPlanSpecifics.Description).To(Equal("plan description"), "maintenance_info.description does not match")
		})

		By("provisioning a service instance with correct maintenance_info", func() {
			url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", brokerInfo.URI, serviceInstanceGUID)
			provisionDetails := ProvisionDetailsWithMaintenanceInfo{
				ServiceID:        serviceID,
				PlanID:           planID,
				OrganizationGUID: "orgId",
				SpaceGUID:        "space",
				MaintenanceInfo:  *maintenanceInfoWithPlanSpecifics,
			}
			resp, bodyContent := doRequest(http.MethodPut, url, provisionDetails)

			Expect(resp.StatusCode).To(Equal(http.StatusAccepted), "provision request status")

			provisioningResponse := apiresponses.ProvisioningResponse{}
			Expect(json.Unmarshal(bodyContent, &provisioningResponse)).To(Succeed())

			pollLastOperation(serviceInstanceGUID, provisioningResponse.OperationData)
		})

		By("successfully upgrading a single service instance to the latest version", func() {
			brokerInfo = bosh.DeployBroker(
				brokerInfo.TestSuffix,
				bosh.BrokerDeploymentOptions{},
				service_helpers.Redis,
				[]string{
					"update_service_catalog.yml",
					"add_lifecycle_errand.yml",
					"update_maintenance_info.yml",
				},
			)

			newPlanSpecificMaintenanceInfo := retrieveCatalog().Services[0].Plans[0].MaintenanceInfo
			updateBody := UpdateBody{
				ServiceID:       serviceID,
				MaintenanceInfo: *newPlanSpecificMaintenanceInfo,
				PlanID:          planID,
				PreviousValues: domain.PreviousValues{
					PlanID:          planID,
					MaintenanceInfo: maintenanceInfoWithPlanSpecifics,
				},
			}

			Expect(len(newPlanSpecificMaintenanceInfo.Public)).ToNot(BeZero(), "maintenance_info.public should not be nil or empty")
			Expect(newPlanSpecificMaintenanceInfo.Private).ToNot(BeZero(), "maintenance_info.private should not be nil or empty")
			Expect(newPlanSpecificMaintenanceInfo.Version).To(Equal("7.8.8-beta"), "maintenance_info.version does not match")
			Expect(newPlanSpecificMaintenanceInfo.Description).To(Equal("rotated creds"), "maintenance_info.description does not match")

			By("accepting the upgrade request", func() {
				url := fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", brokerInfo.URI, serviceInstanceGUID)
				response, bodyContent := doRequest(http.MethodPatch, url, updateBody)
				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				updateResponse := apiresponses.UpdateResponse{}
				Expect(json.Unmarshal(bodyContent, &updateResponse)).To(Succeed())

				pollLastOperation(serviceInstanceGUID, updateResponse.OperationData)
			})

			By("running the post deploy errands", func() {
				tasks := bosh.TasksForDeployment(broker.InstancePrefix + serviceInstanceGUID)
				Expect(tasks).To(HaveLen(3))
				Expect(tasks[0].Description).To(HavePrefix("run errand health-check"), "Post-deploy errand wasn't run")
				Expect(tasks[1].Description).To(ContainSubstring("deploy"), "Expected service instance to have been redeployed")
			})
		})
	})
})

func doRequest(method, url string, reqBody interface{}) (*http.Response, []byte) {
	jsonBody, err := json.Marshal(reqBody)
	Expect(err).NotTo(HaveOccurred())
	body := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequest(method, url, body)
	Expect(err).ToNot(HaveOccurred())

	req.SetBasicAuth(brokerInfo.BrokerUsername, brokerInfo.BrokerPassword)
	req.Header.Set("X-Broker-API-Version", "2.14")

	req.Close = true
	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	Expect(resp.Body.Close()).To(Succeed())
	return resp, bodyContent
}

func retrieveCatalog() apiresponses.CatalogResponse {
	resp, bodyContent := doRequest(http.MethodGet, "http://"+brokerInfo.URI+"/v2/catalog", nil)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	catalogResp := apiresponses.CatalogResponse{}
	err := json.Unmarshal(bodyContent, &catalogResp)
	Expect(err).NotTo(HaveOccurred(), "Error unmarshalling "+string(bodyContent))
	return catalogResp
}

func deleteService(serviceId, planId, serviceInstanceGUID string) {
	resp, bodyContent := doRequest(http.MethodDelete,
		"http://"+brokerInfo.URI+"/v2/service_instances/"+serviceInstanceGUID+"?service_id="+serviceId+"&plan_id="+planId+"&accepts_incomplete=true", nil)
	Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	deprovisionResponse := apiresponses.DeprovisionResponse{}
	err := json.Unmarshal(bodyContent, &deprovisionResponse)
	Expect(err).NotTo(HaveOccurred())
	pollLastOperation(serviceInstanceGUID, deprovisionResponse.OperationData)
}

func pollLastOperation(instanceID, operationData string) {
	sleeps := 0
	for {
		resp, bodyContent := doLastOperationRequest(instanceID, operationData)
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

func doLastOperationRequest(instanceID, operationData string) (*http.Response, []byte) {
	lastOperationURL := fmt.Sprintf("http://%s/v2/service_instances/%s/last_operation", brokerInfo.URI, instanceID)
	lastOperationURL = fmt.Sprintf("%s?operation=%s", lastOperationURL, url.QueryEscape(operationData))

	return doRequest(http.MethodGet, lastOperationURL, nil)
}
