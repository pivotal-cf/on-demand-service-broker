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
			Expect(hashed).To(Equal("ef939ae1b4f1d77a1085fa660d02c06ea159accc441d9243da7fede8401f89b5"))
			Expect(hashed).To(HaveLen(64))

			m["other_key"] = "othervalue"
			Expect(hashed).NotTo(Equal(subject.Hash(m)))
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
