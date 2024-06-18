package manifestsecrets_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
)

var _ = Describe("ManifestSecrets", func() {
	Context("manager construction", func() {
		When("resolve secrets is enabled", func() {
			It("returns a credhub manager", func() {
				Expect(manifestsecrets.BuildManager(true, nil, nil)).To(BeAssignableToTypeOf(new(manifestsecrets.BoshCredHubSecretManager)))
			})
		})

		When("resolve secrets is not enabled", func() {
			It("returns a noop manager", func() {
				Expect(manifestsecrets.BuildManager(false, nil, nil)).To(BeAssignableToTypeOf(new(manifestsecrets.NoopSecretManager)))
			})
		})
	})

	Context("bosh credhub manager", func() {
		var (
			fakeMatcher         *fakes.FakeMatcher
			fakeCredhubOperator *fakes.FakeCredhubOperator
			manager             broker.ManifestSecretManager
		)

		BeforeEach(func() {
			fakeMatcher = new(fakes.FakeMatcher)
			fakeCredhubOperator = new(fakes.FakeCredhubOperator)
			manager = manifestsecrets.BuildManager(true, fakeMatcher, fakeCredhubOperator)
		})

		Describe("ResolveManifestSecrets", func() {
			It("calls the dependent components as expected", func() {
				manifestWithSecrets := []byte("name: ((/some/path))\nvariables:\n-name: /some/var\n  type: password")
				secrets := map[string]boshdirector.Variable{"/some/path": {Path: "/some/path"}}
				secretsValues := map[string]string{"/some/path": "supers3cret"}

				fakeMatcher.MatchReturns(secrets, nil)
				fakeCredhubOperator.BulkGetReturns(secretsValues, nil)

				secretsMap, err := manager.ResolveManifestSecrets(manifestWithSecrets, []boshdirector.Variable{{Path: "/some/var", ID: "1234"}}, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeMatcher.MatchCallCount()).To(Equal(1))

				Expect(fakeCredhubOperator.BulkGetCallCount()).To(Equal(1))
				secretsToFetch, _ := fakeCredhubOperator.BulkGetArgsForCall(0)
				Expect(secretsToFetch).To(Equal(secrets))

				Expect(secretsMap).To(Equal(secretsValues))
			})

			It("returns an error when the matcher errors", func() {
				fakeMatcher.MatchReturns(nil, errors.New("matcher error"))
				_, err := manager.ResolveManifestSecrets([]byte("name: foo"), nil, nil)
				Expect(err).To(MatchError(ContainSubstring("matcher error")))
			})

			It("returns an error when secrets fetcher errors", func() {
				fakeCredhubOperator.BulkGetReturns(map[string]string{}, errors.New("something failed"))
				_, err := manager.ResolveManifestSecrets([]byte("name: foo"), nil, nil)
				Expect(err).To(MatchError(ContainSubstring("something failed")))
			})
		})

		Describe("DeleteSecretsForInstance", func() {
			It("calls the dependent components as expected", func() {
				paths := []string{
					"/odb/some-id/some-instance/foo",
					"/odb/some-id/some-instance/bar",
				}
				expectedInstanceID := "some-instance"

				fakeCredhubOperator.FindNameLikeReturns(paths, nil)
				fakeCredhubOperator.BulkDeleteReturns(nil)

				err := manager.DeleteSecretsForInstance(expectedInstanceID, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCredhubOperator.FindNameLikeCallCount()).To(Equal(1), "expected to call FindNameLike once")
				actualInstanceID, _ := fakeCredhubOperator.FindNameLikeArgsForCall(0)
				Expect(actualInstanceID).To(Equal(expectedInstanceID))

				Expect(fakeCredhubOperator.BulkDeleteCallCount()).To(Equal(1), "expected to call BulkDelete once")
				actualPaths, _ := fakeCredhubOperator.BulkDeleteArgsForCall(0)
				Expect(actualPaths).To(Equal(paths))
			})

			It("returns an error when finding credentials fails", func() {
				fakeCredhubOperator.FindNameLikeReturns(nil, errors.New("FindNameLike failed miserably"))
				err := manager.DeleteSecretsForInstance("foo", nil)
				Expect(err).To(MatchError("FindNameLike failed miserably"))
			})

			It("returns an error when deleting credentials fails", func() {
				fakeCredhubOperator.BulkDeleteReturns(errors.New("BulkDelete failed miserably this time"))
				err := manager.DeleteSecretsForInstance("foo", nil)
				Expect(err).To(MatchError("BulkDelete failed miserably this time"))
			})
		})
	})
})
