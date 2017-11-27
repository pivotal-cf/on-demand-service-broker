// Package gunstructured provides gomega matchers for using unstructured with the ginkgo and
// gomega testing libraries:
//
// https://github.com/onsi/gomega
// https://github.com/onsi/ginkgo
package gunstructured

import (
	"fmt"
	"reflect"

	"github.com/totherme/unstructured"
)

// DataTypeMatcher is a gomega matcher which tests if a given value represents
// json data of a given type.
type DataTypeMatcher struct {
	typ string
}

// BeAnObject returns a gomega matcher which tests if a given value represents
// a json object.
func BeAnObject() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataOb,
	}
}

// BeAnObject returns a gomega matcher which tests if a given value represents
// a json string.
func BeAString() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataString,
	}
}

// BeAList returns a gomega matcher which tests if a given value represents
// a json list.
func BeAList() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataList,
	}
}

// BeANum returns a gomega matcher which tests if a given value represents
// a json num.
func BeANum() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataNum,
	}
}

// BeABool returns a gomega matcher which tests if a given value represents
// a json bool.
func BeABool() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataBool,
	}
}

// BeNull returns a gomega matcher which tests if a given value represents
// json null.
func BeANull() DataTypeMatcher {
	return DataTypeMatcher{
		typ: unstructured.DataNull,
	}
}

// Match is the gomega function that actually checks if the given value is of
// the appropriate json type.
func (m DataTypeMatcher) Match(actual interface{}) (success bool, err error) {
	switch json := actual.(type) {
	default:
		return false, fmt.Errorf("actual is not a Data -- actually of type %s", reflect.TypeOf(actual))
	case unstructured.Data:
		return json.IsOfType(m.typ), nil
	}
}

// FailureMessage constructs a hopefully-helpful error message in the case that
// the given value is not of the appropriate json type.
func (m DataTypeMatcher) FailureMessage(actual interface{}) (message string) {
	if reflect.TypeOf(actual) != reflect.TypeOf(unstructured.Data{}) {
		return fmt.Sprintf("expected a Data object -- got a %s", reflect.TypeOf(actual))
	}

	json := actual.(unstructured.Data)
	for _, t := range []string{unstructured.DataBool,
		unstructured.DataString,
		unstructured.DataNum,
		unstructured.DataList,
		unstructured.DataNull,
		unstructured.DataOb} {
		if json.IsOfType(t) {
			return fmt.Sprintf("expected a Data %s -- got a Data %s", m.typ, t)
		}
	}

	return fmt.Sprintf("expected a Data %s -- got some other crazy kind of Data", m.typ)
}

// NegatedFailureMessage constructs a hopefully-helpful error message in the
// case that the given value is unexpectedly of the appropriate json type.
func (m DataTypeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("got a Data %s, but expected not to", m.typ)
}
