package manifestsecrets_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestManifestSecrets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManifestSecrets Suite")
}
