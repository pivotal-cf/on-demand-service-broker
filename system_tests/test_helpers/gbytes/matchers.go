package gbytes

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

func AnySay(regex string) types.GomegaMatcher {
	return SatisfyAny(
		WithTransform(func(session *gexec.Session) *gbytes.Buffer { return session.Out }, gbytes.Say(regex)),
		WithTransform(func(session *gexec.Session) *gbytes.Buffer { return session.Err }, gbytes.Say(regex)),
	)
}
