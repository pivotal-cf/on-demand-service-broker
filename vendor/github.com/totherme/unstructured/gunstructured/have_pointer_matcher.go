package gunstructured

import (
	"fmt"

	"github.com/totherme/unstructured"
)

// HaveJSONPointerMatcher is a gomega matcher which tests if a given value
// represents a json object containing a particular json pointer.
type HaveJSONPointerMatcher struct {
	p string
}

// HaveJSONPointer returns a gomega matcher  which tests if a given value
// represents a json object containing a given json pointer `p`.
//
// For more information on json pointers see
// https://tools.ietf.org/html/rfc6901
func HaveJSONPointer(p string) HaveJSONPointerMatcher {
	return HaveJSONPointerMatcher{p: p}
}

// Match is the gomega function that actually checks if the given value
// represents a json object containing the particular pointer.
func (m HaveJSONPointerMatcher) Match(actual interface{}) (bool, error) {

	switch t := actual.(type) {
	default:
		return false, fmt.Errorf("not a Data object. Have you done unstructured.Parse[JSON|YAML](...)?")
	case unstructured.Data:
		return t.HasPointer(m.p)
	}
}

// FailureMessage constructs a hopefully-helpful error message in the case that
// the given value does not represent a json object containing the particular
// pointer.
func (m HaveJSONPointerMatcher) FailureMessage(actual interface{}) (message string) {
	actualString := fmt.Sprintf("%+v", actual)
	return fmt.Sprintf("expected '%s' to be a unstructured.Data object with pointer '%s'",
		truncateString(actualString),
		m.p)
}

// NegatedFailureMessage constructs a hopefully-helpful error message in the
// case that the given value unexpectedly represents a json object containing
// the particular pointer.
func (m HaveJSONPointerMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	actualString := fmt.Sprintf("%+v", actual)
	return fmt.Sprintf("expected '%s' not to contain the pointer '%s'",
		truncateString(actualString),
		m.p)
}
