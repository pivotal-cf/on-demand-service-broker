package hasher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/hasher"
)

var _ = Describe("MapHasher", func() {
	var subject *hasher.MapHasher

	BeforeEach(func() {
		subject = new(hasher.MapHasher)
	})

	Describe("Hash()", func() {
		It("hashes the map", func() {
			m := map[string]string{"key": "value"}

			hashed := subject.Hash(m)
			Expect(hashed).To(Equal("81dc6f83e3da3843e6d79fc3cf866ed57e2a50fc89e1879053bef112dc889512"))
			Expect(hashed).To(HaveLen(64))

			m = map[string]string{"key": "other-value"}
			Expect(hashed).NotTo(Equal(subject.Hash(m)), "Changes in values should change the hash result")

			m = map[string]string{"key2": "value"}
			Expect(hashed).NotTo(Equal(subject.Hash(m)), "Changes in keys should change the hash result")

			m = map[string]string{"key": "value"}
			Expect(hashed).To(Equal(subject.Hash(m)), "An identical map should generate the same hash")
		})

		It("returns empty when the map is nil", func() {
			hashed := subject.Hash(nil)
			Expect(hashed).To(Equal(""))
		})

		It("generates the same output for the same map", func() {
			m := map[string]string{"key": "value", "other": "val", "mooo": "123"}

			hashed := subject.Hash(m)
			Consistently(func() string { return subject.Hash(m) }).Should(Equal(hashed))
		})
	})
})
