package cf_test

import (
	"io"
	"log"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

var _ = Describe("/v2/info contract", func() {
	var (
		subject *cf.Client
		logger  *log.Logger
	)

	BeforeEach(func() {
		logBuffer := gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
		subject = NewCFClient(logger)
	})

	It("has the API version", func() {
		version, err := subject.GetAPIVersion(logger)

		Expect(err).NotTo(HaveOccurred())
		_, err = semver.Parse(version)
		Expect(err).NotTo(HaveOccurred())
	})

	It("has the OSBAPI version", func() {
		Expect(subject.CheckMinimumOSBAPIVersion("0.0.1", logger)).To(BeTrue())
		Expect(subject.CheckMinimumOSBAPIVersion("999999999.0.0", logger)).To(BeFalse())
	})
})
