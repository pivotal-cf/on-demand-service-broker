package gbytes

import (
	"fmt"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

type anySayMatcher struct {
	outMatcher               types.GomegaMatcher
	errMatcher               types.GomegaMatcher
	failureMessageOut        string
	failureMessageErr        string
	negatedFailureMessageOut string
	negatedFailureMessageErr string
}

func AnySay(regex string) types.GomegaMatcher {
	return &anySayMatcher{
		outMatcher: gbytes.Say(regex),
		errMatcher: gbytes.Say(regex),
	}
}

func (m *anySayMatcher) Match(actual interface{}) (success bool, errOut error) {
	session, ok := actual.(*gexec.Session)
	if !ok {
		return false, fmt.Errorf("expected to match on a session")
	}

	foundOut, errOut := m.outMatcher.Match(session.Out)
	if errOut != nil {
		return foundOut, errOut
	}
	if foundOut {
		m.negatedFailureMessageOut = m.outMatcher.NegatedFailureMessage(session.Out)
		return foundOut, errOut
	} else {
		m.failureMessageOut = m.outMatcher.FailureMessage(session.Out)
	}

	foundErr, errErr := m.errMatcher.Match(session.Err)
	if errErr != nil {
		return foundErr, errErr
	}
	if foundErr {
		m.negatedFailureMessageErr = m.errMatcher.NegatedFailureMessage(session.Err)
	} else {
		m.failureMessageErr = m.errMatcher.FailureMessage(session.Err)
	}
	return foundErr, errErr

}

func (m *anySayMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to match on STDOUT or STDERR.\nSTDOUT:\n%s\n\nSTDERR:\n%s\n",
		m.failureMessageOut, m.failureMessageErr)
}

func (m *anySayMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not match on STDOUT or STDERR.\nSTDOUT:\n%s\n\nSTDERR:\n%s\n",
		m.negatedFailureMessageOut, m.negatedFailureMessageErr)
}
