package gunstructured

import (
	"fmt"

	"github.com/totherme/unstructured"
)

// HaveJSONKeyMatcher is a gomega matcher which tests if a given value
// represents a json object containing a particular key.
type HaveJSONKeyMatcher struct {
	key string
}

// HaveJSONKey returns a gomega matcher  which tests if a given value
// represents an unstructured object containing a given `key`.
func HaveJSONKey(key string) HaveJSONKeyMatcher {
	return HaveJSONKeyMatcher{key: key}
}

// HaveYAMLKey is exactly like HaveJSONKey
func HaveYAMLKey(key string) HaveJSONKeyMatcher {
	return HaveJSONKeyMatcher{key: key}
}

// Match is the gomega function that actually checks if the given value
// represents a json object containing the particular key.
func (m HaveJSONKeyMatcher) Match(actual interface{}) (bool, error) {
	switch j := actual.(type) {
	default:
		return false, fmt.Errorf("not a Data object. Have you done unstructured.Parse[JSON|YAML](...)?")
	case unstructured.Data:
		return j.HasKey(m.key), nil
	}
}

// FailureMessage constructs a hopefully-helpful error message in the case that
// the given value does not represent a json object containing the particular
// key.
func (m HaveJSONKeyMatcher) FailureMessage(actual interface{}) (message string) {
	actualString := fmt.Sprintf("%+v", actual)
	return fmt.Sprintf("expected '%s' to be an unstructured.Data object with key '%s'",
		truncateString(actualString),
		m.key)
}

// NegatedFailureMessage constructs a hopefully-helpful error message in the
// case that the given value unexpectedly represents a json object containing
// the particular key.
func (m HaveJSONKeyMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	actualString := fmt.Sprintf("%+v", actual)
	return fmt.Sprintf("expected '%s' not to contain the key '%s'",
		truncateString(actualString),
		m.key)
}

func truncateString(s string) (t string) {
	if len(s) > 50 {
		t = fmt.Sprintf("%s...", s[0:50])
	} else {
		t = s
	}
	return
}
