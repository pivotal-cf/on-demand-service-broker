package on_demand_service_broker_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
)

const (
	serverCertFile   = "../fixtures/mybroker.crt"
	nonsenseCertFile = "../fixtures/nonsense.crt"
	serverKeyFile    = "../fixtures/mybroker.key"
	caCertFile       = "../fixtures/bosh.ca.crt"
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

			err := StartServer(conf)
			Expect(err).NotTo(HaveOccurred())
		})

		It("serves HTTPS", func() {
			response, bodyContent, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, acceptableCipherSuites, tls.VersionTLS12)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			catalog := make(map[string][]domain.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
			Expect(catalog["services"][0].Name).To(Equal(serviceName))
		})

		DescribeTable("can use the desired cipher suites",
			func(cipher uint16) {
				response, _, err := doHTTPSRequest(http.MethodGet, fmt.Sprintf("https://%s/v2/catalog", serverURL), caCertFile, []uint16{cipher}, tls.VersionTLS12)
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

	Describe("with HTTPS configured badly", func() {
		var (
			conf brokerConfig.Config
		)

		BeforeEach(func() {
			conf = brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

		})

		It("should error when certificate is invalid", func() {
			conf.Broker.TLS = brokerConfig.TLSConfig{
				CertFile: nonsenseCertFile,
				KeyFile:  serverKeyFile,
			}
			err := StartServer(conf)

			Expect(err).To(MatchError(ContainSubstring("failed to find any PEM data in certificate input")))
		})

		It("should error when certificate is expired", func() {
			privateKey, expiredCert := generateCertificateExpiringOn(time.Now().Add(-time.Hour * 24))

			privateKeyFile, err := ioutil.TempFile("", "privateKey")
			Expect(err).NotTo(HaveOccurred())
			written, err := privateKeyFile.Write(privateKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(written).To(Equal(len(privateKey)))
			defer os.Remove(privateKeyFile.Name())

			expiredCertFile, err := ioutil.TempFile("", "serverCert")
			Expect(err).NotTo(HaveOccurred())
			written, err = expiredCertFile.Write(expiredCert)
			Expect(err).NotTo(HaveOccurred())
			Expect(written).To(Equal(len(expiredCert)))
			defer os.Remove(expiredCertFile.Name())

			conf.Broker.TLS = brokerConfig.TLSConfig{
				CertFile: expiredCertFile.Name(),
				KeyFile:  privateKeyFile.Name(),
			}
			err = StartServer(conf)

			Expect(err).To(MatchError(ContainSubstring("server certificate expired on")))
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
			response, bodyContent := doRequestWithAuthAndHeaderSet(http.MethodGet, fmt.Sprintf("http://%s/v2/catalog", serverURL), nil)

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			catalog := make(map[string][]domain.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
			Expect(catalog["services"][0].Name).To(Equal(serviceName))
		})
	})
})

func generateCertificateExpiringOn(expiry time.Time) (privKey, serverCert []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	var privateBlock = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privBytes := pem.EncodeToMemory(privateBlock)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"foo"},
		},
		NotAfter: expiry,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	Expect(err).ToNot(HaveOccurred())

	cert := &bytes.Buffer{}
	pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return privBytes, cert.Bytes()
}
