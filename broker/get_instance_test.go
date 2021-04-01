package broker_test

import (
	"context"
	"github.com/pivotal-cf/brokerapi/v8/domain"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
)

var _ = Describe("GetInstance", func() {
	It("returns an appropriate error", func() {
		_, err := b.GetInstance(context.Background(), "some-instance-id", domain.FetchInstanceDetails{PlanID: "test plan", ServiceID: "test service"})
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(logger)).To(Equal(404))
	})
})
