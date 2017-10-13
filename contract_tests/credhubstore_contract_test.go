package contract_tests

import (
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
)

var _ = Describe("Credential store", func() {
	BehavesLikeACredentialStore := func(correctAuth, incorrectAuth, noUAAConfig credhubbroker.CredentialStore) {
		It("deletes and sets a key-value credential", func() {
			err := correctAuth.Delete("new-name")
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("does not exist"))
			}
			err = correctAuth.Set("new-name", map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())
		})

		It("can store plain string values", func() {
			err := correctAuth.Delete("stringy-cred")
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("does not exist"))
			}
			err = correctAuth.Set("stringy-cred", "I JUST LOVE CREDENTIALS.")
			Expect(err).NotTo(HaveOccurred())
		})

		It("produces error when storing other types", func() {
			err := correctAuth.Delete("esoteric-type-of-cred")
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("does not exist"))
			}
			err = correctAuth.Set("esoteric-type-of-cred", []interface{}{"asdf"})
			Expect(err).To(MatchError("Unknown credential type"))
		})

		It("produces error when authenticating late without UAA config", func() {
			err := noUAAConfig.Delete("doesn't-really-matter")
			Expect(err.Error()).To(ContainSubstring("invalid_token"))
		})

		It("produces error when authenticating early without UAA config", func() {
			err := noUAAConfig.Authenticate()
			Expect(err).To(HaveOccurred())
		})

		It("produces error with incorrect credentials", func() {
			err := incorrectAuth.Authenticate()
			Expect(err).To(HaveOccurred())
		})
	}

	credhubCorrectAuth := func() credhubbroker.CredentialStore {
		clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
		if clientSecret == "" {
			panic("Expected TEST_CREDHUB_CLIENT_SECRET to be set")
		}
		credentialStore, err := credhubbroker.NewCredHubStore(
			"https://credhub.service.cf.internal:8844",
			credhub.SkipTLSValidation(true),
			credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
		)
		if err != nil {
			panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
		}
		return credentialStore
	}

	credhubIncorrectAuth := func() credhubbroker.CredentialStore {
		credentialStore, err := credhubbroker.NewCredHubStore(
			"https://credhub.service.cf.internal:8844",
			credhub.SkipTLSValidation(true),
			credhub.Auth(auth.UaaClientCredentials("credhub_cli", "reallybadsecret")),
		)
		if err != nil {
			panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
		}
		return credentialStore
	}

	credhubNoUAAConfig := func() credhubbroker.CredentialStore {
		credentialStore, err := credhubbroker.NewCredHubStore(
			"https://credhub.service.cf.internal:8844",
			credhub.SkipTLSValidation(true),
		)
		if err != nil {
			panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
		}
		return credentialStore
	}

	if _, ok := os.LookupEnv("TEST_CREDHUB_CLIENT_SECRET"); ok {
		BehavesLikeACredentialStore(credhubCorrectAuth(), credhubIncorrectAuth(), credhubNoUAAConfig())
	} else {
		log.Println("SKIPPING 'REAL' CREDHUB CONTRACT TEST - set TEST_CREDHUB_CLIENT_SECRET to enable")
	}
})
