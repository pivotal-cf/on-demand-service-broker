package feature_toggled_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFeatureToggled(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureToggled Suite")
}
