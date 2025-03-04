package broker_test

import (
	"context"
	"log/slog"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetInstance", func() {
	It("returns an appropriate error", func() {
		_, err := b.GetInstance(context.Background(), "some-instance-id", domain.FetchInstanceDetails{PlanID: "test plan", ServiceID: "test service"})
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(slog.New(lager.NewHandler(logger)))).To(Equal(404))
	})
})
