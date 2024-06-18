package network_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/network"
)

var _ = Describe("AppendCertsFromPEM", func() {
	var (
		systemPool     *x509.CertPool
		certificatePEM string
		subjectName    string
	)

	BeforeEach(func() {
		systemPool, _ = x509.SystemCertPool()
		subjectName = "UnitTestCertificates"
		certificatePEM = generateCertificate(subjectName)
	})

	It("appends a certificate to the system pool", func() {
		certPool, err := network.AppendCertsFromPEM(certificatePEM)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(certPool.Subjects())).To(Equal(len(systemPool.Subjects()) + 1))

		subjects := certPool.Subjects()
		lastSubject := subjects[len(subjects)-1]

		Expect(string(lastSubject)).To(ContainSubstring(subjectName))
	})

	It("accepts multiple certificates", func() {
		certPool, err := network.AppendCertsFromPEM(certificatePEM, generateCertificate("anotherSubject"))
		Expect(err).ToNot(HaveOccurred())

		Expect(len(certPool.Subjects())).To(Equal(len(systemPool.Subjects()) + 2))
	})

	It("returns the system pool when called with an empty certificate", func() {
		certPool, err := network.AppendCertsFromPEM("")
		Expect(err).ToNot(HaveOccurred())

		Expect(len(certPool.Subjects())).To(Equal(len(systemPool.Subjects())))
	})
})

func generateCertificate(subjectOrgName string) string {
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{subjectOrgName},
		},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	Expect(err).ToNot(HaveOccurred())
	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return out.String()
}
