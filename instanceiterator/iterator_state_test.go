// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instanceiterator_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Iterate State", func() {
	It("fails if canary instances is not a subset of all the instances", func() {
		_, err := instanceiterator.NewIteratorState([]service.Instance{service.Instance{GUID: "a"}}, []service.Instance{}, 0)
		Expect(err).To(MatchError(ContainSubstring("Canary 'a' not in")))
	})

	Context("when processing canaries", func() {
		It("says it is processing canaries", func() {
			canaries, all := instances(func(i int) bool {
				return i%2 == 1
			}, 10)
			us, err := instanceiterator.NewIteratorState(canaries, all, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(us.IsProcessingCanaries()).To(BeTrue())
		})

		It("can retrieve the next pending canary", func() {
			canaries, all := instances(func(i int) bool {
				return i%2 == 1
			}, 10)
			us, err := instanceiterator.NewIteratorState(canaries, all, 0)
			Expect(err).NotTo(HaveOccurred())

			canary, err := us.NextPending()
			Expect(err).NotTo(HaveOccurred())
			Expect(canary.GUID).To(Equal("guid_1"))

			err = us.SetState(canaries[1].GUID, instanceiterator.OperationAccepted)
			Expect(err).NotTo(HaveOccurred())

			canary, err = us.NextPending()
			Expect(err).NotTo(HaveOccurred())
			Expect(canary.GUID).To(Equal("guid_5"))
		})

		It("starts in canary mode when new'ed up with canaries", func() {
			canaries, all := instances(func(i int) bool {
				return i%2 == 1
			}, 2)
			us, err := instanceiterator.NewIteratorState(canaries, all, 0)
			Expect(err).NotTo(HaveOccurred())
			next, err := us.NextPending()
			Expect(err).NotTo(HaveOccurred())
			Expect(next.GUID).To(Equal("guid_1"))
		})

		It("NextPending() fails with an error if there are no more canaries available", func() {
			canaries, all := instances(func(i int) bool {
				return i%2 == 1
			}, 2)
			us, err := instanceiterator.NewIteratorState(canaries, all, 0)
			Expect(err).NotTo(HaveOccurred())
			us.SetState(canaries[0].GUID, instanceiterator.OperationAccepted)

			_, err = us.NextPending()
			Expect(err).To(MatchError("Cannot retrieve next pending instance"))
		})

		It("can list instances in a certain state", func() {
			canaries, all := instances(func(i int) bool {
				return i%2 == 1
			}, 10)

			us, err := instanceiterator.NewIteratorState(canaries, all, 0)
			Expect(err).NotTo(HaveOccurred())

			us.SetState(all[0].GUID, instanceiterator.OperationAccepted)
			us.SetState(all[3].GUID, instanceiterator.OperationAccepted)
			us.SetState(all[5].GUID, instanceiterator.OperationFailed)

			instances := us.GetInstancesInStates(instanceiterator.OperationAccepted, instanceiterator.OperationFailed)
			Expect(instances).To(Equal([]service.Instance{all[3], all[5]}))
		})
	})

	It("can set the state of an instance", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 2)
		us, err := instanceiterator.NewIteratorState(canaries, all, 0)
		Expect(err).NotTo(HaveOccurred())

		err = us.SetState(canaries[0].GUID, instanceiterator.OperationAccepted)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Completion", func() {
		It("returns that the process is completed when there are no more pending instances", func() {
			us, err := instanceiterator.NewIteratorState([]service.Instance{}, []service.Instance{}, 0)
			Expect(err).NotTo(HaveOccurred())

			Expect(us.CurrentPhaseIsComplete()).To(Equal(true))
		})

		DescribeTable("process completed when processing canaries",
			func(limit, complete int, expected bool) {
				canaries, all := instances(func(i int) bool { return i < 3 }, 10)
				us, err := instanceiterator.NewIteratorState(canaries, all, limit)
				Expect(err).NotTo(HaveOccurred())

				for i := 0; i < complete; i++ {
					us.SetState(fmt.Sprintf("guid_%d", i), instanceiterator.OperationSucceeded)
				}

				Expect(us.CurrentPhaseIsComplete()).To(Equal(expected))
			},
			Entry("with limit 1, completed 1", 1, 1, true),
			Entry("with limit 2, completed 1", 2, 1, false),
			Entry("with limit 2, completed 3", 2, 3, true),
			Entry("with limit 0, completed 1", 0, 1, false),
			Entry("with limit 0, completed 3", 0, 3, true),
		)

		DescribeTable("process completed when processing all the rest",
			func(complete int, expected bool) {
				canaries, all := instances(func(i int) bool { return i%3 == 0 }, 10)
				us, err := instanceiterator.NewIteratorState(canaries, all, 7)
				Expect(err).NotTo(HaveOccurred())

				us.MarkCanariesCompleted()

				for i := 0; i < complete; i++ {
					us.SetState(fmt.Sprintf("guid_%d", i), instanceiterator.OperationSucceeded)
				}

				Expect(us.CurrentPhaseIsComplete()).To(Equal(expected))
			},
			Entry("with completed 10", 10, true),
			Entry("with completed 0", 0, false),
			Entry("with completed 5", 5, false),
		)
	})

	It("can pick any pending instance when not processing canaries", func() {
		canaries, all := instances(func(i int) bool {
			return i%2 == 1
		}, 4)
		us, err := instanceiterator.NewIteratorState(canaries, all, 0)
		Expect(err).NotTo(HaveOccurred())
		next, err := us.NextPending()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_1"))
		us.SetState(next.GUID, instanceiterator.OperationAccepted)

		us.MarkCanariesCompleted()

		next, err = us.NextPending()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_0"))

		By("skipping the done canary and getting the next non-canary then the next canary")
		_, err = us.NextPending()
		Expect(err).NotTo(HaveOccurred())
		next, err = us.NextPending()
		Expect(err).NotTo(HaveOccurred())
		Expect(next.GUID).To(Equal("guid_3"))
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
