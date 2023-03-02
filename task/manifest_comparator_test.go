package task

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest Comparator", func() {
	Context("when comparing valid manifests", func() {
		It("returns true when the content is exactly the same", func() {
			manifest := "name: a-manifest"

			result, err := ManifestsAreTheSame([]byte(manifest), []byte(manifest))

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
		})

		It("returns true even when the values are the same", func() {
			manifest := "name: a-manifest"
			manifestWithTrailingSpaces := manifest + "    "

			result, err := ManifestsAreTheSame([]byte(manifest), []byte(manifestWithTrailingSpaces))

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
		})

		It("returns false when the values are different", func() {
			manifest := "name: a-manifest"
			anotherManifest := "name: another-manifest"

			result, err := ManifestsAreTheSame([]byte(manifest), []byte(anotherManifest))

			Expect(err).To(Not(HaveOccurred()))
			Expect(result, err).To(BeFalse())
		})

		It("returns true when the only the update value is different", func() {
			manifest := `
name: a-manifest
update: 
  canaries: 1
  max_in_flight: 1`
			anotherManifest := `
name: a-manifest
update: 
  canaries: 999
  max_in_flight: 1`

			result, err := ManifestsAreTheSame([]byte(manifest), []byte(anotherManifest))

			Expect(err).To(Not(HaveOccurred()))
			Expect(result, err).To(BeTrue())
		})
	})

	Context("when comparing invalid manifests", func() {
		It("returns false and an error when the first manifest is invalid", func() {
			invalidManifest := "sjondfs. esdifnjk. not a valid yaml...."

			result, err := ManifestsAreTheSame([]byte(invalidManifest), []byte("name: a-manifest"))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to unmarshal manifest"))
			Expect(result).To(BeFalse())
		})

		It("returns false and an error when the second manifest is invalid", func() {
			invalidManifest := "sjondfs. esdifnjk. not a valid yaml...."

			result, err := ManifestsAreTheSame([]byte("name: a-manifest"), []byte(invalidManifest))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to unmarshal manifest"))
			Expect(result).To(BeFalse())
		})
	})
})
