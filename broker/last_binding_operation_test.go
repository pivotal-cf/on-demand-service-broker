package broker_test

import (
	"context"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/brokerapi"
)

var _ = Describe("LastBindingOperation", func() {

	It("returns an appropriate error", func() {
		_, err := b.LastBindingOperation(context.Background(), instanceID, "bID", brokerapi.PollDetails{})
		fresp, ok := err.(*brokerapi.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(logger)).To(Equal(404))
	})
})
