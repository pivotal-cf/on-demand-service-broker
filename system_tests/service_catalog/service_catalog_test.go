package service_catalog_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
)

var _ = Describe("Version Info in the Service Catalog", func() {

	It("returns the global and plan version info combined for a plan", func() {
		req, err := http.NewRequest(http.MethodGet, "http://"+brokerURI+"/v2/catalog", nil)
		Expect(err).ToNot(HaveOccurred())

		req.SetBasicAuth("broker", brokerPassword)
		req.Header.Set("X-Broker-API-Version", "2.14")
		req.Close = true

		resp, err := http.DefaultClient.Do(req)
		Expect(err).ToNot(HaveOccurred())

		bodyContent, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.Body.Close()).To(Succeed())

		type CatalogResp struct {
			Services []struct {
				Plans []struct {
					MaintenanceInfo struct {
						Public struct {
							Name            string `json:"name"`
							StemcellVersion string `json:"stemcell_version"`
							VMType          string `json:"vm_type"`
						} `json:"public"`
					} `json:"maintenance_info"`
				} `json:"plans"`
			} `json:"services"`
		}

		var catalogResp = CatalogResp{}

		err = json.Unmarshal(bodyContent, &catalogResp)
		Expect(err).NotTo(HaveOccurred(), "Error unmarshalling "+string(bodyContent))

		Expect(len(catalogResp.Services[0].Plans)).To(Equal(1))

		Expect(catalogResp.Services[0].Plans[0].MaintenanceInfo.Public.Name).To(Equal("dolores"))
		Expect(catalogResp.Services[0].Plans[0].MaintenanceInfo.Public.StemcellVersion).To(Equal("1234"))
		Expect(catalogResp.Services[0].Plans[0].MaintenanceInfo.Public.VMType).To(Equal("small"))
	})
})
