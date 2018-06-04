package credstore_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credstore"
	"github.com/pivotal-cf/on-demand-service-broker/credstore/fakes"
)

var _ = Describe("Operations", func() {
	Describe("BulkGet", func() {
		var (
			fakeGetter     *fakes.FakeCredhubGetter
			ops            *credstore.Operations
			secretsToFetch [][]byte
		)

		BeforeEach(func() {
			fakeGetter = new(fakes.FakeCredhubGetter)
			ops = credstore.New(fakeGetter)
			secretsToFetch = [][]byte{
				[]byte("/some/path"),
				[]byte("/some/otherpath"),
			}
		})

		It("can read secrets from bosh credhub", func() {
			secretsMap := map[string]string{
				"/some/path":      "thesecret",
				"/some/otherpath": "someothersec",
			}

			creds1 := credentials.Credential{
				Value: "thesecret",
			}

			creds2 := credentials.Credential{
				Value: "someothersec",
			}

			fakeGetter.GetLatestVersionReturnsOnCall(0, creds1, nil)
			fakeGetter.GetLatestVersionReturnsOnCall(1, creds2, nil)

			secrets, err := ops.BulkGet(secretsToFetch)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGetter.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))
			for i, p := range secretsToFetch {
				Expect(fakeGetter.GetLatestVersionArgsForCall(i)).To(Equal(string(p)))
			}

			Expect(secrets).To(Equal(secretsMap))
		})

		It("errors when cannot get latest version", func() {
			fakeGetter.GetLatestVersionReturnsOnCall(0, credentials.Credential{}, errors.New("failed to get secret"))

			_, err := ops.BulkGet(secretsToFetch)
			Expect(err).To(MatchError(ContainSubstring("failed to get secret")))
		})

		It("errors when cannot marshal the credential", func() {
			fakeGetter.GetLatestVersionReturnsOnCall(0, credentials.Credential{Value: make(chan int)}, nil)

			_, err := ops.BulkGet(secretsToFetch)
			Expect(err).To(MatchError(ContainSubstring("failed to marshal")))
		})

		It("deals with non-string secrets", func() {
			secretsMap := map[string]string{
				"/some/path":      `{"foo":"bar"}`,
				"/some/otherpath": "someothersec",
			}

			creds1 := credentials.Credential{
				Value: values.JSON{
					"foo": "bar",
				},
			}

			creds2 := credentials.Credential{
				Value: "someothersec",
			}

			fakeGetter.GetLatestVersionReturnsOnCall(0, creds1, nil)
			fakeGetter.GetLatestVersionReturnsOnCall(1, creds2, nil)

			secrets, err := ops.BulkGet(secretsToFetch)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGetter.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))
			for i, p := range secretsToFetch {
				Expect(fakeGetter.GetLatestVersionArgsForCall(i)).To(Equal(string(p)))
			}

			Expect(secrets).To(Equal(secretsMap))
		})
	})
})
