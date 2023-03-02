package manifestsecrets_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("ODB Secrets", func() {
	var (
		subject           manifestsecrets.ODBSecrets
		serviceOfferingID string
	)
	BeforeEach(func() {
		serviceOfferingID = "jimbob"
		subject = manifestsecrets.ODBSecrets{
			ServiceOfferingID: serviceOfferingID,
		}
	})

	Describe("GenerateSecretPaths", func() {
		var (
			generatedManifestSecrets serviceadapter.ODBManagedSecrets
		)

		BeforeEach(func() {
			generatedManifestSecrets = serviceadapter.ODBManagedSecrets{
				"foo":    "bar",
				"secret": "value",
			}
		})

		It("generates a list of ManifestSecrets that are present in the manifest", func() {
			deploymentName := "the-name"
			manifest := "name: ((odb_secret:foo))\npassword: ((odb_secret:secret))"
			generatedManifestSecrets["not_in_manifest"] = "blah"
			secretsPath := subject.GenerateSecretPaths(deploymentName, manifest, generatedManifestSecrets)
			Expect(secretsPath).To(HaveLen(2))
			Expect(secretsPath).To(SatisfyAll(
				ContainElement(broker.ManifestSecret{Name: "foo", Path: fmt.Sprintf("/odb/%s/%s/foo", serviceOfferingID, deploymentName), Value: generatedManifestSecrets["foo"]}),
				ContainElement(broker.ManifestSecret{Name: "secret", Path: fmt.Sprintf("/odb/%s/%s/secret", serviceOfferingID, deploymentName), Value: generatedManifestSecrets["secret"]}),
			))
		})

		It("returns an empty list of ManifestSecrets when there are no matches in the manifest", func() {
			deploymentName := "the-name"
			manifest := "name: foo"
			secretsPath := subject.GenerateSecretPaths(deploymentName, manifest, generatedManifestSecrets)
			Expect(secretsPath).To(BeEmpty())
		})
	})

	Describe("ReplaceODBRefs", func() {
		It("replaces odb_secret:foo", func() {
			manifest := fmt.Sprintf("name: ((%s:foo))\nsecret: ((%[1]s:bar))", serviceadapter.ODBSecretPrefix)
			secrets := []broker.ManifestSecret{
				{Name: "foo", Value: "something", Path: "/odb/jim/bob/foo"},
				{Name: "bar", Value: "another thing", Path: "/odb/jim/bob/bar"},
			}
			expectedManifest := "name: ((/odb/jim/bob/foo))\nsecret: ((/odb/jim/bob/bar))"
			substitutedManifest := subject.ReplaceODBRefs(manifest, secrets)
			Expect(substitutedManifest).To(Equal(expectedManifest))
		})

		It("replaces all occurrences of a managed secret", func() {
			manifest := fmt.Sprintf("name: ((%s:foo))\nsecret: ((%[1]s:foo))", serviceadapter.ODBSecretPrefix)
			secrets := []broker.ManifestSecret{
				{Name: "foo", Value: "something", Path: "/odb/jim/bob/foo"},
			}
			expectedManifest := "name: ((/odb/jim/bob/foo))\nsecret: ((/odb/jim/bob/foo))"
			substitutedManifest := subject.ReplaceODBRefs(manifest, secrets)
			Expect(substitutedManifest).To(Equal(expectedManifest))
		})
	})
})
