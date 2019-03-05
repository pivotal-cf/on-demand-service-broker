package cf_helpers

import (
	"regexp"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func GetOAuthToken() string {
	cmd := Cf("oauth-token")
	Eventually(cmd, CfTimeout).Should(gexec.Exit(0))
	oauthTokenOutput := string(cmd.Buffer().Contents())
	oauthTokenRe := regexp.MustCompile(`(?m)^bearer .*$`)
	authToken := oauthTokenRe.FindString(oauthTokenOutput)
	Expect(authToken).ToNot(BeEmpty())
	return authToken
}
