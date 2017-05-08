package network_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"time"
)

var _ = Describe("DefaultHttpClient", func() {
	It("has a client timeout", func() {
		client := network.NewDefaultHTTPClient()
		Expect(client.Timeout).To(Equal(30 * time.Second))
	})
})
