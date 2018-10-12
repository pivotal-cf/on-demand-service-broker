package on_demand_service_broker_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
)

const (
	serverCertFile = "./assets/mybroker.crt"
	serverKeyFile  = "./assets/mybroker.key"
	caCertFile     = "./assets/bosh.ca.crt"
)

var _ = Describe("Server Protocol", func() {
	Describe("with HTTPS configured", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					TLS: brokerConfig.TLSConfig{
						CertFile: serverCertFile,
						KeyFile:  serverKeyFile,
					},
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			StartServer(conf)
		})
		It("serves HTTPS", func() {
			response, bodyContent := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), nil, caCertFile)

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			catalog := make(map[string][]brokerapi.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
		})

		It("does not serve HTTP", func() {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/v2/catalog", serverURL), nil)
			Expect(err).ToNot(HaveOccurred())

			req.SetBasicAuth(brokerUsername, brokerPassword)
			req.Header.Set("X-Broker-API-Version", "2.14")
			req.Close = true

			_, err = http.DefaultClient.Do(req)
			Expect(err).To(MatchError(ContainSubstring("HTTP/1.x transport connection broken: malformed HTTP response")))
		})
	})

	Describe("with HTTP configured", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			StartServer(conf)
		})
		It("serves HTTP", func() {
			response, bodyContent := doRequest(http.MethodGet, fmt.Sprintf("http://%s/v2/catalog", serverURL), nil)

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			catalog := make(map[string][]brokerapi.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
		})
	})
})
