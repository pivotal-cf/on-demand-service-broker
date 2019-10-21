package updateparser_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-cf/brokerapi/domain"
	. "github.com/pivotal-cf/on-demand-service-broker/broker/updateparser"
)

var _ = Describe("Update Parser", func() {
	DescribeTable("Updates vs Upgrades",
		func(parser UpdateParser, expectedUpgrade bool, expectedError error) {
			upgrade, err := parser.IsUpgrade()

			if expectedError != nil {
				Expect(err).To(MatchError(expectedError))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(upgrade).To(Equal(expectedUpgrade))
			}
		},

		Entry(
			"no MaintenanceInfo on the request, previous state, or plan",
			UpdateParser{},
			false,
			nil,
		),

		Entry(
			"same MaintenanceInfo on request and plan, but different on previous state",
			UpdateParser{
				Details: UpdateDetails{
					MaintenanceInfo: &MaintenanceInfo{Version: "2.0.0"},
					PreviousValues: PreviousValues{
						MaintenanceInfo: &MaintenanceInfo{Version: "1.0.0"},
					},
				},
				PlanMaintenanceInfo: &MaintenanceInfo{Version: "2.0.0"},
			},
			true,
			nil,
		),

		Entry(
			"MaintenanceInfo on request is different to plan",
			UpdateParser{
				Details: UpdateDetails{
					MaintenanceInfo: &MaintenanceInfo{Version: "1.0.0"},
				},
				PlanMaintenanceInfo: &MaintenanceInfo{Version: "2.0.0"},
			},
			false,
			fmt.Errorf("plan error"),
		),
	)
})
