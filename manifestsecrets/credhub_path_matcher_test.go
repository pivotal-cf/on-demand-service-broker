package manifestsecrets_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
)

var _ = Describe("CredhubPathMatcher", func() {

	Describe("Match", func() {
		When("manifest has no variables block", func() {
			It("matches all variables", func() {
				manifest := []byte(`---
foo: ((/path/to/one))
bar:
  sha: ((/path/to/two))
  quux: ((relative/path))
another: ((couldBeAVarButIsnt))
`)
				expectedMatches := [][]byte{
					[]byte("/path/to/one"),
					[]byte("/path/to/two"),
					[]byte("relative/path"),
					[]byte("couldBeAVarButIsnt"),
				}

				matcher := new(manifestsecrets.CredHubPathMatcher)
				matches, err := matcher.Match(manifest)

				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(Equal(expectedMatches))
			})
		})

		When("manifest has a variables block", func() {
			It("ignores the variables in the block", func() {
				manifest := []byte(`---
foo: ((/path/to/one))
bar:
  sha: ((/path/to/two))
  quux: ((relative/path))
another: ((isAVar))
variables:
- name: isAVar
  type: password
`)
				expectedMatches := [][]byte{
					[]byte("/path/to/one"),
					[]byte("/path/to/two"),
					[]byte("relative/path"),
				}

				matcher := new(manifestsecrets.CredHubPathMatcher)
				matches, err := matcher.Match(manifest)

				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(Equal(expectedMatches))
			})
		})

		It("returns an error when manifest is invalid", func() {
			manifest := []byte(`what`)
			matcher := new(manifestsecrets.CredHubPathMatcher)
			_, err := matcher.Match(manifest)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	Describe("NamesFromVarsBlock", func() {
		It("returns a map of variable names in the variables block", func() {
			manifest := []byte(`---
foo: ((/path/to/one))
bar:
  sha: ((/path/to/two))
  quux: ((relative/path))
another: ((/isAVar))
variables:
- name: isAVar
  type: password
- name: boo
  type: certificate
`)
			matcher := new(manifestsecrets.CredHubPathMatcher)
			variables, err := matcher.NamesFromVarsBlock(manifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(variables)).To(Equal(2))
			Expect(variables["isAVar"]).To(BeTrue())
			Expect(variables["boo"]).To(BeTrue())
		})

		It("errors when the manifest is an invalid yaml", func() {
			manifest := []byte(`what`)
			matcher := new(manifestsecrets.CredHubPathMatcher)
			_, err := matcher.NamesFromVarsBlock(manifest)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})

		It("errors when the variables block is invalid", func() {
			manifest := []byte(`{ "variables" : [{"type":"password"}] }`)
			matcher := new(manifestsecrets.CredHubPathMatcher)
			_, err := matcher.NamesFromVarsBlock(manifest)
			Expect(err).To(MatchError(ContainSubstring("variable without name in variables block")))
		})
	})

	When("when there are two vars in a single line", func() {
		It("both will be found", func() {
			manifest := []byte("name: ((foo))stuff((bar))")
			matcher := new(manifestsecrets.CredHubPathMatcher)
			matches, err := matcher.Match(manifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(ConsistOf([][]byte{
				[]byte("foo"),
				[]byte("bar"),
			}))
		})
	})
})
