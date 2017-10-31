package credhub_tests

import (
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
)

var _ = Describe("Credential store", func() {
	It("sets and deletes a key-value map credential", func() {
		keyPath := makeKeyPath("new-name")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, map[string]interface{}{"hi": "there"})
		Expect(err).NotTo(HaveOccurred())

		err = correctAuth.Delete(keyPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("can store plain string values", func() {
		keyPath := makeKeyPath("stringy-cred")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, "I JUST LOVE CREDENTIALS.")
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces error when storing other types", func() {
		keyPath := makeKeyPath("esoteric-cred")
		correctAuth := credhubCorrectAuth()
		err := correctAuth.Set(keyPath, []interface{}{"asdf"})
		Expect(err).To(MatchError("Unknown credential type"))
	})

	It("produces error when authenticating late without UAA config", func() {
		keyPath := makeKeyPath("doesnt-really-matter")
		noUAAConfig := credhubNoUAAConfig()
		err := noUAAConfig.Delete(keyPath)
		Expect(err.Error()).To(ContainSubstring("invalid_token"))
	})

	It("produces error when authenticating early without UAA config", func() {
		noUAAConfig := credhubNoUAAConfig()
		err := noUAAConfig.Authenticate()
		Expect(err).To(HaveOccurred())
	})

	It("produces error with incorrect credentials", func() {
		incorrectAuth := credhubIncorrectAuth()
		err := incorrectAuth.Authenticate()
		Expect(err).To(HaveOccurred())
	})

	Describe("CredHub credential store", func() {
		It("can't be constructed with a bad URI", func() {
			_, err := credhubbroker.NewCredHubStore("💩://hi.there#you", credhub.SkipTLSValidation(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot contain colon"))
		})
	})
})
