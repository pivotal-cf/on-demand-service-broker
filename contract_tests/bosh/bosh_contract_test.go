package bosh_test

import (
	"fmt"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BOSH client", func() {
	var director director.Director

	BeforeEach(func() {
		director = getDirector()
	})

	It("talks to the director", func() {
		info, err := director.Info()
		Expect(err).NotTo(HaveOccurred())

		Expect(info.Name).To(Equal("bosh-lite-director"))
	})

	It("get the info about director", func() {
		info, err := director.Info()
		fmt.Println(info)
		Expect(err).NotTo(HaveOccurred())

		uaaURL, ok := info.Auth.Options["url"].(string)
		Expect(ok).To(BeTrue(), "Cannot retrieve UAA url from /info")

		Expect(uaaURL).To(Equal("https://35.189.248.241:8443"))
	})

	It("is an authenticated director", func() {
		isAuth, err := director.IsAuthenticated()
		Expect(isAuth).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})
})
