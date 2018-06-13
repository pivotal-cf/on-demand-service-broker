package manifestsecrets_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
)

var _ = Describe("CredhubPathMatcher", func() {
	Describe("Match", func() {
		It("correctly matches the deployment variables", func() {
			manifest := []byte(`---
name: cocoon
another: ((isAVar))
foo: ((/path/to/one))
bar:
  sha: ((/path/to/two))
  quux: ((relative/path))
  yo: ((/other/absolute/path))
  yo: ((/absolute/path))
  fo: ((2isAVar))
  relative: ((relative))
variables:
- name: isAVar
  type: password
- name: /other/absolute/path
  type: password
- name: /absolute/path
  type: password
- name: 2isAVar
  type: password
`)
			expectedMatches := map[string]boshdirector.Variable{
				"/path/to/one":         {Path: "/path/to/one"},
				"/path/to/two":         {Path: "/path/to/two"},
				"relative/path":        {Path: "relative/path"},
				"/other/absolute/path": {Path: "/other/absolute/path", ID: "yet-another-id"},
				"2isAVar":              {Path: "/baboon/cocoon/2isAVar", ID: "the-id"},
				"isAVar":               {Path: "/baboon/cocoon/isAVar", ID: "some-id"},
				"/absolute/path":       {Path: "/absolute/path", ID: "some-other-id"},
				"relative":             {Path: "/baboon/cocoon/relative", ID: "relative-id"},
			}

			matcher := new(manifestsecrets.CredHubPathMatcher)
			matches, err := matcher.Match(manifest, []boshdirector.Variable{
				{Path: "/other/absolute/path", ID: "yet-another-id"},
				{Path: "/baboon/cocoon/2isAVar", ID: "the-id"},
				{Path: "/baboon/cocoon/isAVar", ID: "some-id"},
				{Path: "/baboon/cocoon/relative", ID: "relative-id"},
				{Path: "/absolute/path", ID: "some-other-id"},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(Equal(expectedMatches))
		})
	})

	When("when there are two vars in a single line", func() {
		It("both will be found", func() {
			manifest := []byte("name: ((foo))stuff((bar))")
			matcher := new(manifestsecrets.CredHubPathMatcher)
			matches, err := matcher.Match(manifest, nil)
			Expect(err).NotTo(HaveOccurred())
			expectedMatches := map[string]boshdirector.Variable{
				"foo": {Path: "foo"},
				"bar": {Path: "bar"},
			}
			Expect(matches).To(Equal(expectedMatches))
		})
	})
})
