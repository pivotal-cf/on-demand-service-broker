package broker_test

import (
	"context"

	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/brokerapi/v9/domain/apiresponses"
)

var _ = Describe("LastBindingOperation", func() {
	It("returns an appropriate error", func() {
		_, err := b.LastBindingOperation(context.Background(), "instanceID", "bID", domain.PollDetails{})
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(logger)).To(Equal(404))
	})
})
