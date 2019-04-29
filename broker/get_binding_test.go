package broker_test

import (
	"context"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

var _ = Describe("GetBinding", func() {
	It("returns an error", func() {
		_, err := b.GetBinding(context.Background(), "instanceID", "bID")
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(logger)).To(Equal(404))
	})
})
