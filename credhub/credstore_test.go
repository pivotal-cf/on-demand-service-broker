package credhub_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/credhub/fakes"
)

var _ = Describe("CredStore", func() {
	var (
		fakeCredhubClient *fakes.FakeCredhubClient
		store             *credhub.Store
	)

	BeforeEach(func() {
		fakeCredhubClient = new(fakes.FakeCredhubClient)
		store = credhub.New(fakeCredhubClient)
	})

	Describe("Bulk Get", func() {
		var (
			secretsToFetch map[string]boshdirector.Variable
		)

		BeforeEach(func() {
			secretsToFetch = map[string]boshdirector.Variable{
				"/some/path":       {Path: "/some/path"},
				"/some/otherpath":  {Path: "/some/otherpath"},
				"/other/path":      {Path: "/other/path", ID: "123"},
				"/other/otherpath": {Path: "/other/otherpath", ID: "456"},
			}
			fakeCredhubClient.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				var found bool
				for _, p := range secretsToFetch {
					if p.Path == path {
						found = true
					}
				}
				Expect(found).To(BeTrue())
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", path)}, nil
			}

			fakeCredhubClient.GetByIdStub = func(id string) (credentials.Credential, error) {
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

			secrets, err := store.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			By("calling GetLatestVersion when there's no ID")
			Expect(fakeCredhubClient.GetLatestVersionCallCount()).To(Equal(2))

			By("calling GetById when the deployment variable ID is present")
			Expect(fakeCredhubClient.GetByIdCallCount()).To(Equal(2))
			Expect(secrets).To(Equal(secretsMap))
		})

		It("errors when cannot get latest version", func() {
			secretsMap := map[string]string{
				"/some/otherpath":  "resolved-for-/some/otherpath",
				"/other/otherpath": "resolved-for-456",
			}

			fakeCredhubClient.GetByIdStub = func(id string) (credentials.Credential, error) {
				if id == "123" {
					return credentials.Credential{}, errors.New("oops-1")
				}
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", id)}, nil
			}
			fakeCredhubClient.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				if path == "/some/path" {
					return credentials.Credential{}, errors.New("oops-2")
				}
				return credentials.Credential{Value: fmt.Sprintf("resolved-for-%s", path)}, nil
			}
			outputBuffer := gbytes.NewBuffer()
			logger := log.New(outputBuffer, "unit-test", log.LstdFlags)
			secrets, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(secrets).To(Equal(secretsMap))
			Expect(string(outputBuffer.Contents())).To(
				SatisfyAll(
					ContainSubstring("oops-1"),
					ContainSubstring("oops-2"),
				))
		})

		It("errors when cannot marshal the credential", func() {
			fakeCredhubClient.GetLatestVersionReturnsOnCall(0, credentials.Credential{Value: make(chan int)}, nil)

			_, err := store.BulkGet(secretsToFetch, nil)
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

			fakeCredhubClient.GetLatestVersionStub = func(path string) (credentials.Credential, error) {
				if path == "/some/path" {
					return creds1, nil
				}
				return creds2, nil
			}

			secrets, err := store.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCredhubClient.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))
			Expect(secrets).To(Equal(secretsMap))
		})

		It("uses the name of the variable as a key in the returned secrets map", func() {
			secretsToFetch = map[string]boshdirector.Variable{
				"from_vars_block": {Path: "/p-bosh/p-deployment/from_vars_block"},
			}
			secretsMap := map[string]string{
				"from_vars_block": "resolved-for-/p-bosh/p-deployment/from_vars_block",
			}

			secrets, err := store.BulkGet(secretsToFetch, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCredhubClient.GetLatestVersionCallCount()).To(Equal(len(secretsToFetch)))

			Expect(secrets).To(Equal(secretsMap))
		})
	})

	Describe("Set", func() {
		It("can set a json secret", func() {
			secret := map[string]interface{}{}
			secret["foo"] = "bar"
			err := store.Set("/path/to/secret", secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.SetJSONCallCount()).To(Equal(1))
			path, val, _ := fakeCredhubClient.SetJSONArgsForCall(0)
			Expect(path).To(Equal("/path/to/secret"))
			Expect(val).To(Equal(values.JSON(secret)))
		})

		It("can set a string secret", func() {
			err := store.Set("/path/to/secret", "caravan")
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.SetValueCallCount()).To(Equal(1))
			path, val, _ := fakeCredhubClient.SetValueArgsForCall(0)
			Expect(path).To(Equal("/path/to/secret"))
			Expect(val).To(Equal(values.Value("caravan")))
		})

		It("errors if not a JSON or string secret", func() {
			err := store.Set("/path/to/secret", make(chan int))
			Expect(err).To(MatchError("Unknown credential type"))
		})
	})

	Describe("Delete", func() {
		It("can delete a credhub secret at path p", func() {
			p := "/some/path"
			store.Delete(p)
			Expect(fakeCredhubClient.DeleteCallCount()).To(Equal(1))
			Expect(fakeCredhubClient.DeleteArgsForCall(0)).To(Equal(p))
		})

		It("returns an error if the underlying call fails", func() {
			fakeCredhubClient.DeleteReturns(errors.New("you what?"))
			err := store.Delete("something")
			Expect(err).To(MatchError("you what?"))
		})
	})

	Describe("Add Permission", func() {
		It("can add permissions to a path", func() {
			p := "/some/path"
			perms := []permissions.Permission{
				{Actor: "jim", Operations: []string{"read", "corrupt"}},
			}
			_, err := store.AddPermissions(p, perms)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.AddPermissionsCallCount()).To(Equal(1))
			actualName, actualPerms := fakeCredhubClient.AddPermissionsArgsForCall(0)
			Expect(actualName).To(Equal(p))
			Expect(actualPerms).To(Equal(perms))
		})

		It("returns an error if the underlying call fails", func() {
			p := "/some/path"
			perms := []permissions.Permission{
				{Actor: "jim", Operations: []string{"read", "corrupt"}},
			}
			fakeCredhubClient.AddPermissionsReturns(nil, errors.New("you're joking, right?"))
			_, err := store.AddPermissions(p, perms)
			Expect(err).To(MatchError("you're joking, right?"))
		})
	})

})
