package broker_test

import (
	"context"
	"github.com/pivotal-cf/brokerapi/v9/domain"

	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v9/domain/apiresponses"
)

var _ = Describe("GetBinding", func() {
	It("returns an error", func() {
		_, err := b.GetBinding(context.Background(), "instanceID", "bID", domain.FetchBindingDetails{ServiceID: "test service", PlanID: "test plan"})
		fresp, ok := err.(*apiresponses.FailureResponse)
		Expect(ok).To(BeTrue(), "err wasn't a FailureResponse")
		logger := lager.NewLogger("test")
		Expect(fresp.ValidatedStatusCode(logger)).To(Equal(404))
	})
})
