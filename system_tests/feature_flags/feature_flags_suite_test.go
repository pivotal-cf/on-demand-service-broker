package feature_flags_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFeatureFlags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureFlags Suite")
}
