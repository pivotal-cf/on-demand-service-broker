package credhub_test

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestCredStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CredStore Suite")
}

func toJson(i interface{}) string {
	b := gbytes.NewBuffer()
	json.NewEncoder(b).Encode(i)
	return strings.TrimRight(string(b.Contents()), "\n")
}
