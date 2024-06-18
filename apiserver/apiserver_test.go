package apiserver_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
)

var _ = Describe("Apiserver", func() {
	DescribeTable("checking certificate expiry", func(expiryDate time.Time, ok bool) {
		_, certPEMBytes := generateCertificateExpiringOn(expiryDate)
		certFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(certFile.Name())
		n, err := certFile.Write(certPEMBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(certPEMBytes)))

		err = apiserver.CheckCertExpiry(certFile.Name())
		if ok {
			Expect(err).ToNot(HaveOccurred())
		} else {
			Expect(err).To(MatchError(ContainSubstring("server certificate expired on")))
		}
	},
		Entry("valid cert", time.Now().Add(time.Hour*24), true),
		Entry("expired cert", time.Now().Add(-time.Hour*24), false),
	)

	Context("certificate file problems", func() {
		It("errors when cert file not found", func() {
			err := apiserver.CheckCertExpiry("/does/not/exist")
			Expect(err).To(MatchError(ContainSubstring("can't read server certificate")))
		})

		It("errors when cert file does not contain a PEM block", func() {
			certFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(certFile.Name())
			invalidContent := []byte("not a valid cert in PEM format")
			n, err := certFile.Write(invalidContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(len(invalidContent)))

			err = apiserver.CheckCertExpiry(certFile.Name())
			Expect(err).To(MatchError(ContainSubstring("failed to find any PEM data in certificate input")))
		})

		It("errors when the PEM block can't be parsed as x509 cert", func() {
			err := apiserver.CheckCertExpiry("fixtures/badcert.pem")
			Expect(err).To(MatchError(ContainSubstring("can't parse server certificate file")))
		})
	})
})

func generateCertificateExpiringOn(expiry time.Time) (privKey, serverCert []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	privateBlock := &pem.Block{
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
