package manifestsecrets_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestManifestSecrets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManifestSecrets Suite")
}
