package broker_test

import (
	"context"
	"log/slog"

	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v12/domain"
	"github.com/pivotal-cf/brokerapi/v12/domain/apiresponses"
)

var _ = Describe("LastBindingOperation", func() {
	It("returns an appropriate error", func() {
		_, err := b.LastBindingOperation(context.Background(), "instanceID", "bID", domain.PollDetails{})
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(slog.New(lager.NewHandler(logger)))).To(Equal(404))
	})
})
