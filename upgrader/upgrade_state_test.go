package upgrader_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
)

var _ = Describe("Upgrade State", func() {
	It("fails if canary instances is not a subset of all the instances", func() {
		_, err := upgrader.NewUpgradeState([]service.Instance{service.Instance{GUID: "a"}}, []service.Instance{}, -1)
		Expect(err).To(MatchError(ContainSubstring("Canary 'a' not in")))
	})

	It("can retrieve the next pending canary", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 10)
		us, err := upgrader.NewUpgradeState(canaries, all, -1)
		Expect(err).NotTo(HaveOccurred())

		canary, err := us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(canary.GUID).To(Equal("guid_1"))

		canary, err = us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(canary.GUID).To(Equal("guid_1"))

		err = us.SetState(canaries[0].GUID, services.UpgradeAccepted)
		Expect(err).NotTo(HaveOccurred())

		canary, err = us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(canary.GUID).To(Equal("guid_3"))
	})

	It("fails with an error if there are no canaries available", func() {
		us, err := upgrader.NewUpgradeState([]service.Instance{}, []service.Instance{}, -1)
		Expect(err).NotTo(HaveOccurred())

		_, err = us.Next()
		Expect(err).To(MatchError("Cannot retrieve next canary instance"))
	})

	It("can set the state of an instance", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 2)
		us, err := upgrader.NewUpgradeState(canaries, all, -1)
		Expect(err).NotTo(HaveOccurred())

		err = us.SetState(canaries[0].GUID, services.UpgradeAccepted)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns that the upgrade is completed when there are no instances", func() {
		us, err := upgrader.NewUpgradeState([]service.Instance{}, []service.Instance{}, -1)
		Expect(err).NotTo(HaveOccurred())

		Expect(us.UpgradeCompleted()).To(Equal(true))
	})

	DescribeTable("processing canaries",
		func(limit, complete int, expected bool) {
			canaries, all := instances(func(i int) bool { return i < 3 }, 10)
			us, err := upgrader.NewUpgradeState(canaries, all, limit)
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < complete; i++ {
				us.SetState(fmt.Sprintf("guid_%d", i), services.UpgradeSucceeded)
			}

			Expect(us.UpgradeCompleted()).To(Equal(expected))
		},
		Entry("with limit 1, completed 1", 1, 1, true),
		Entry("with limit 2, completed 1", 2, 1, false),
		Entry("with limit 2, completed 3", 2, 3, true),
		Entry("with limit 0, completed 1", 0, 1, false),
		Entry("with limit 0, completed 3", 0, 3, true),
	)

	DescribeTable("processing all or the rest",
		func(complete int, expected bool) {
			canaries, all := instances(func(i int) bool { return i%3 == 0 }, 10)
			us, err := upgrader.NewUpgradeState(canaries, all, 7)
			Expect(err).NotTo(HaveOccurred())

			us.MarkCanariesCompleted()

			for i := 0; i < complete; i++ {
				us.SetState(fmt.Sprintf("guid_%d", i), services.UpgradeSucceeded)
			}

			Expect(us.UpgradeCompleted()).To(Equal(expected))
		},
		Entry("with completed 10", 10, true),
		Entry("with completed 0", 0, false),
		Entry("with completed 5", 5, false),
	)

	It("returns a canaries next when new'ed up with canaries", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 2)
		us, err := upgrader.NewUpgradeState(canaries, all, -1)
		Expect(err).NotTo(HaveOccurred())
		next, err := us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_1"))
	})

	It("can pick all the instances if is not processing canaries", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 2)
		us, err := upgrader.NewUpgradeState(canaries, all, -1)
		Expect(err).NotTo(HaveOccurred())
		next, err := us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_1"))

		us.MarkCanariesCompleted()
		next, err = us.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_0"))
	})

})

func instances(isCanary func(int) bool, total int) (canaries []service.Instance, all []service.Instance) {
	for i := 0; i < total; i++ {
		inst := service.Instance{GUID: fmt.Sprintf("guid_%d", i), PlanUniqueID: "plan"}
		all = append(all, inst)
		if isCanary(i) {
			canaries = append(canaries, inst)
		}
	}
	return
}
