package cf_helpers

import (
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/onsi/gomega/gexec"
)

const RETRY_LIMIT = 3
const COMMAND_TIMEOUT = CfTimeout

func CfWithTimeout(timeout time.Duration, args ...string) *gexec.Session {
	session := cf.Cf(args...)

	select {
	case <-session.Exited:
	case <-time.After(COMMAND_TIMEOUT):
		session.Kill().Wait()
	}
	return session
}

func Cf(args ...string) *gexec.Session {
	var s *gexec.Session
	for i := 0; i < RETRY_LIMIT; i++ {
		s = CfWithTimeout(COMMAND_TIMEOUT, args...)
		if s.ExitCode() == 0 {
			return s
		}
	}
	return s
}
