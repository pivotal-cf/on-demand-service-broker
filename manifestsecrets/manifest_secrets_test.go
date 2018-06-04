package manifestsecrets_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
)

var _ = Describe("ManifestSecrets", func() {

	Context("resolver construction", func() {
		When("resolve secrets is enabled", func() {
			It("returns a credhub resolver", func() {
				Expect(manifestsecrets.NewResolver(true, nil, nil)).To(BeAssignableToTypeOf(new(manifestsecrets.BoshCredHubSecretResolver)))
			})
		})

		When("resolve secrets is not enabled", func() {
			It("returns a noop resolver", func() {
				Expect(manifestsecrets.NewResolver(false, nil, nil)).To(BeAssignableToTypeOf(new(manifestsecrets.NoopSecretResolver)))
			})
		})
	})

	Context("bosh credhub resolver", func() {
		When("processing manifest", func() {
			var (
				fakeMatcher    *fakes.FakeMatcher
				fakeBulkGetter *fakes.FakeBulkGetter
				resolver       broker.ManifestSecretResolver
			)

			BeforeEach(func() {
				fakeMatcher = new(fakes.FakeMatcher)
				fakeBulkGetter = new(fakes.FakeBulkGetter)
				resolver = manifestsecrets.NewResolver(true, fakeMatcher, fakeBulkGetter)
			})

			It("calls the dependent components as expected", func() {
				manifestWithSecrets := []byte("name: ((/some/path))")
				secrets := [][]byte{
					[]byte("/some/path"),
				}
				secretsValues := map[string]string{
					"/some/path": "supers3cret",
				}

				fakeMatcher.MatchReturns(secrets, nil)
				fakeBulkGetter.BulkGetReturns(secretsValues, nil)

				secretsMap, err := resolver.ResolveManifestSecrets(manifestWithSecrets)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeMatcher.MatchCallCount()).To(Equal(1))

				Expect(fakeBulkGetter.BulkGetCallCount()).To(Equal(1))
				secretsToFetch := fakeBulkGetter.BulkGetArgsForCall(0)
				Expect(secretsToFetch).To(Equal(secrets))

				Expect(secretsMap).To(Equal(secretsValues))
			})

			It("returns an error when the matcher errors", func() {
				fakeMatcher.MatchReturns(nil, errors.New("matcher error"))
				_, err := resolver.ResolveManifestSecrets([]byte("name: foo"))
				Expect(err).To(MatchError(ContainSubstring("matcher error")))
			})

			It("returns an error when secrets fetcher errors", func() {
				fakeBulkGetter.BulkGetReturns(map[string]string{}, errors.New("something failed"))
				_, err := resolver.ResolveManifestSecrets([]byte("name: foo"))
				Expect(err).To(MatchError(ContainSubstring("something failed")))
			})
		})
	})

})
