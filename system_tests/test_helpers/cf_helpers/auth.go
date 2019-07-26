package cf_helpers

import (
	"regexp"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func GetOAuthToken() string {
	session := Cf("oauth-token")
	Expect(session).To(gexec.Exit(0))
	oauthTokenOutput := string(session.Buffer().Contents())
	oauthTokenRe := regexp.MustCompile(`(?m)^bearer .*$`)
	authToken := oauthTokenRe.FindString(oauthTokenOutput)
	Expect(authToken).ToNot(BeEmpty())
	return authToken
}
