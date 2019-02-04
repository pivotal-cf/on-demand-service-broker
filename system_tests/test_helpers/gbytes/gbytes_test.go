package gbytes_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/gbytes"
)

var _ = Describe("gbytes.AnySay", func() {

	var (
		session *gexec.Session
	)

	Context("passing something other than a gexec.Session", func() {
		It("should return an error", func() {
			_, err := gbytes.AnySay("foo").Match("foo")
			Expect(err).To(MatchError("expected to match on a session"))
		})
	})

	Context("when stdout matches", func() {

		BeforeEach(func() {
			cmd := exec.Command("/bin/bash", "-c", "echo foo")
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit())
		})

		It("returns true", func() {
			res, err := gbytes.AnySay("foo").Match(session)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeTrue())
		})

	})

	Context("when neither matches", func() {

		BeforeEach(func() {
			cmd := exec.Command("/bin/bash", "-c", "echo bar; echo sha >& 2")
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit())
		})

		It("returns false", func() {
			matcher := gbytes.AnySay("foo")
			res, err := matcher.Match(session)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeFalse())

			message := matcher.FailureMessage(session)
			expectedMessage := `Expected to match on STDOUT or STDERR.
STDOUT:
Got stuck at:
    bar
    
Waiting for:
    foo

STDERR:
Got stuck at:
    sha
    
Waiting for:
    foo`
			Expect(message).To(ContainSubstring(expectedMessage))
		})

	})

	Context("when stderr matches", func() {

		BeforeEach(func() {
			cmd := exec.Command("/bin/bash", "-c", "echo foo >& 2")
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit())
		})

		It("returns true", func() {
			res, err := gbytes.AnySay("foo").Match(session)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeTrue())
		})

	})

	Context("negated failure messages", func() {

		BeforeEach(func() {
			cmd := exec.Command("/bin/bash", "-c", "echo foo ; echo bar >& 2")
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit())
		})

		It("stdout unexpected match returns correct message", func() {
			matcher := gbytes.AnySay("foo")
			_, err := matcher.Match(session)
			Expect(err).NotTo(HaveOccurred())

			expectedMessage := `Expected to not match on STDOUT or STDERR.
STDOUT:
Saw:
    foo
    
Which matches the unexpected:
    foo

STDERR:
`
			Expect(matcher.NegatedFailureMessage(session)).To(ContainSubstring(expectedMessage))
		})

		It("stdout unexpected match returns correct message", func() {
			matcher := gbytes.AnySay("bar")
			_, err := matcher.Match(session)
			Expect(err).NotTo(HaveOccurred())

			expectedMessage := `Expected to not match on STDOUT or STDERR.
STDOUT:


STDERR:
Saw:
    bar
    
Which matches the unexpected:
    bar
`
			Expect(matcher.NegatedFailureMessage(session)).To(ContainSubstring(expectedMessage))
		})

	})
})
