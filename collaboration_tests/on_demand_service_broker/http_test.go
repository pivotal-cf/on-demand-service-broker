package on_demand_service_broker_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
)

const (
	serverCertFile = "../fixtures/mybroker.crt"
	serverKeyFile  = "../fixtures/mybroker.key"
	caCertFile     = "../fixtures/bosh.ca.crt"
)

var acceptableCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

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
			response, bodyContent, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, acceptableCipherSuites, 0)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			catalog := make(map[string][]brokerapi.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
			Expect(catalog["services"][0].Name).To(Equal(serviceName))
		})

		DescribeTable("can use the desired cipher suites",
			func(cipher uint16) {
				response, _, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, []uint16{cipher}, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.TLS.CipherSuite).To(Equal(cipher))
			},
			// The first two cipher suites that Pivotal recommends are not available in Golang
			// Entry("TLS_DHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_DHE_RSA_WITH_AES_128_GCM_SHA256),
			// Entry("TLS_DHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_DHE_RSA_WITH_AES_256_GCM_SHA384),
			Entry("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
			Entry("TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
		)

		DescribeTable("does not serve when the client uses an unacceptable cipher",
			func(cipher uint16) {
				log.SetOutput(GinkgoWriter)
				_, _, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, []uint16{cipher}, 0)
				log.SetOutput(os.Stdout)
				Expect(err).To(MatchError(ContainSubstring("remote error: tls: handshake failure")))
			},
			Entry("TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305),
			Entry("TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA),
			Entry("TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256),
		)

		It("refuses to serve TLS 1.1", func() {
			log.SetOutput(GinkgoWriter)
			_, _, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, acceptableCipherSuites, tls.VersionTLS11)
			log.SetOutput(os.Stdout)
			Expect(err).To(MatchError(ContainSubstring("remote error: tls: protocol version not supported")))
		})

		It("does not serve HTTP", func() {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/v2/catalog", serverURL), nil)
			Expect(err).ToNot(HaveOccurred())

			log.SetOutput(GinkgoWriter)
			resp, err := http.DefaultClient.Do(req)
			log.SetOutput(os.Stdout)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
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
			Expect(catalog["services"][0].Name).To(Equal(serviceName))
		})
	})
})
