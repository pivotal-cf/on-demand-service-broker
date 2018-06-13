package credstore_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/credstore"
	"github.com/pivotal-cf/on-demand-service-broker/credstore/fakes"
)

var _ = Describe("Operations", func() {
	Describe("BulkGet", func() {
		var (
			fakeGetter     *fakes.FakeCredhubGetter
			ops            *credstore.Operations
			secretsToFetch map[string]boshdirector.Variable
		)

		BeforeEach(func() {
			fakeGetter = new(fakes.FakeCredhubGetter)
			ops = credstore.New(fakeGetter)
			secretsToFetch = map[string]boshdirector.Variable{
				"/some/path":       {Path: "/some/path"},
				"/some/otherpath":  {Path: "/some/otherpath"},
				"/other/path":      {Path: "/other/path", ID: "123"},
				"/other/otherpath": {Path: "/other/otherpath", ID: "456"},
			}

			fakeGetter.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				var found bool
				for _, p := range secretsToFetch {
					if p.Path == path {
						found = true
					}
				}
				Expect(found).To(BeTrue())
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", path)}, nil
			}

			fakeGetter.GetByIdStub = func(id string) (credentials.Credential, error) {
				var found bool
				for _, p := range secretsToFetch {
					if p.ID == id {
						found = true
					}
				}
				Expect(found).To(BeTrue())
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", id)}, nil
			}
		})

		It("can read secrets from bosh credhub", func() {
			secretsMap := map[string]string{
				"/some/otherpath":  "resolved-for-/some/otherpath",
				"/some/path":       "resolved-for-/some/path",
				"/other/path":      "resolved-for-123",
				"/other/otherpath": "resolved-for-456",
			}

			secrets, err := ops.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			By("calling GetLatestVersion when there's no ID")
			Expect(fakeGetter.GetLatestVersionCallCount()).To(Equal(2))

			By("calling GetById when the deployment variable ID is present")
			Expect(fakeGetter.GetByIdCallCount()).To(Equal(2))
			Expect(secrets).To(Equal(secretsMap))
		})

		It("continue fetching secrets even if some secrets could not be resolved", func() {
			secretsMap := map[string]string{
				"/some/otherpath":  "resolved-for-/some/otherpath",
				"/other/otherpath": "resolved-for-456",
			}

			fakeGetter.GetByIdStub = func(id string) (credentials.Credential, error) {
				if id == "123" {
					return credentials.Credential{}, errors.New("oops-1")
				}
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", id)}, nil
			}
			fakeGetter.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				if path == "/some/path" {
					return credentials.Credential{}, errors.New("oops-2")
				}
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", path)}, nil
			}
			outputBuffer := gbytes.NewBuffer()
			logger := log.New(outputBuffer, "unit-test", log.LstdFlags)
			secrets, err := ops.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(secrets).To(Equal(secretsMap))
			Expect(string(outputBuffer.Contents())).To(
				SatisfyAll(
					ContainSubstring("oops-1"),
					ContainSubstring("oops-2"),
				))
		})

		It("errors when cannot marshal the credential", func() {
			fakeGetter.GetLatestVersionReturnsOnCall(0, credentials.Credential{Value: make(chan int)}, nil)

			_, err := ops.BulkGet(secretsToFetch, nil)
			Expect(err).To(MatchError(ContainSubstring("failed to marshal")))
		})

		It("deals with non-string secrets", func() {
			secretsToFetch = map[string]boshdirector.Variable{
				"/some/path":      {Path: "/some/path"},
				"/some/otherpath": {Path: "/some/otherpath"},
			}
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

			fakeGetter.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				if path == "/some/path" {
					return creds1, nil
				}
				return creds2, nil
			}

			secrets, err := ops.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGetter.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))
			Expect(secrets).To(Equal(secretsMap))
		})

		It("uses the name of the variable as a key in the returned secrets map", func() {
			secretsToFetch = map[string]boshdirector.Variable{
				"from_vars_block": {Path: "/p-bosh/p-deployment/from_vars_block"},
			}
			secretsMap := map[string]string{
				"from_vars_block": "resolved-for-/p-bosh/p-deployment/from_vars_block",
			}

			secrets, err := ops.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGetter.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))

			Expect(secrets).To(Equal(secretsMap))
		})
	})
})
