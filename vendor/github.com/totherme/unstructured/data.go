// Package unstructured provides ways of manipulating unstructured data such as JSON or YAML
package unstructured

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/xeipuuv/gojsonpointer"
)

const (
	DataString = "string"
	DataNum    = "number"
	DataOb     = "object"
	DataList   = "list"
	DataNull   = "null"
	DataBool   = "bool"
)

// Data represents some unstructured data
type Data struct {
	data interface{}
}

// ParseYAML unmarshals yaml from an input string. Use this for generating a
// Data struct, whose contents you can examine using the following functions.
func ParseYAML(rawjson string) (Data, error) {
	jsonbytes, err := yaml.YAMLToJSON([]byte(rawjson))
	if err != nil {
		return Data{}, err
	}
	return ParseJSON(string(jsonbytes))
}

// ParseJSON unmarshals json from an input string. Use this for generating a
// Data struct, whose contents you can examine using the following functions.
func ParseJSON(rawjson string) (Data, error) {
	j := Data{}
	err := json.Unmarshal([]byte(rawjson), &j.data)
	if err != nil {
		return Data{}, fmt.Errorf("parse error: %s", err.Error())
	}
	return j, nil
}

// IsOb returns true iff the data represented by this Data struct is an object
// or map.
func (j Data) IsOb() bool {
	return reflect.TypeOf(j.data) == reflect.TypeOf(map[string]interface{}{})
}

// UnsafeObValue returns a golang map[string]interface{} represenation of the object
// represented by this Data struct. If the Data struct does not represent an
// object, this method panics. If in doubt, check with `IsOb()`
func (j Data) UnsafeObValue() map[string]interface{} {
	return j.data.(map[string]interface{})
}

// ObValue returns a golang map[string]interface{} represenation of the object
// represented by this Data struct. If the Data struct does not represent an
// object, this method returns an error
func (j Data) ObValue() (map[string]interface{}, error) {
	if !j.IsOb() {
		return nil, fmt.Errorf("This is not an object, so we can't get the object value of it")
	}
	return j.UnsafeObValue(), nil
}

// HasKey returns true iff the object represented by this Data struct contains `key`
//
// Note: this will panic if the data represented by this Data struct is not an
// object. If in doubt, check with `IsOb()`
func (j Data) HasKey(key string) bool {
	jmap := j.data.(map[string]interface{})
	_, ok := jmap[key]
	return ok
}

// HasPointer returns true iff the object represented by this Data struct
// contains the pointer `p`
//
// For more information on json pointers, see https://tools.ietf.org/html/rfc6901
func (j Data) HasPointer(p string) (bool, error) {
	pointer, err := gojsonpointer.NewJsonPointer(p)
	if err != nil {
		return false, err
	}
	_, _, err = pointer.Get(j.data)
	return err == nil, nil
}

// GetByPointer returns a Data struct containing the contents of the original
// data at the given pointer address `p`.
// For more information on json pointers, see https://tools.ietf.org/html/rfc6901
func (j Data) GetByPointer(p string) (data Data, err error) {

	pointer, err := gojsonpointer.NewJsonPointer(p)
	if err != nil {
		return
	}
	json, _, err := pointer.Get(j.data)
	data = Data{data: json}
	return
}

// UnsafeGetField returns a Data struct containing the contents of the original data
// at the given `key`. If this method name feels too long, use `F(key)`.
//
// Note: this function panics if the given `key` does not exist. If in doubt,
// check with `HasKey()`.
func (j Data) UnsafeGetField(key string) Data {
	jmap := j.data.(map[string]interface{})
	val, ok := jmap[key]
	if !ok {
		panic("getting a non-existing field from a Data")
	}
	return Data{data: val}
}

// F is a shorthand for `UnsafeGetField`
func (j Data) F(key string) Data {
	return j.UnsafeGetField(key)
}

// Keys returns a list of the keys on this Data object.
//
// If this is not a Data object, return an error
func (j Data) Keys() ([]string, error) {
	if !j.IsOb() {
		return nil, fmt.Errorf("This is not an object, so you can't get a list of its keys.")
	}
	jmap := j.data.(map[string]interface{})
	var keys []string
	for key := range jmap {
		keys = append(keys, key)
	}
	return keys, nil
}

// SetField updates the field `fieldName` of this Data object.
// If the field `fieldName` does not exist on this object, create it.
//
// If this Data does not represent an object, return an error.
func (j Data) SetField(fieldName string, val interface{}) error {
	if !j.IsOb() {
		return fmt.Errorf("This is not an object, so you can't set a field on it.")
	}
	jmap := j.data.(map[string]interface{})
	jmap[fieldName] = val

	return nil
}

// RawValue returns the raw go value of the parsed data, without any type
// checking
func (j Data) RawValue() interface{} {
	return j.data
}

// IsString returns true iff the data represented by this Data struct is a
// string.
func (j Data) IsString() bool {
	return reflect.TypeOf(j.data) == reflect.TypeOf("")
}

// UnsafeStringValue returns the golang string representation of the string
// represented by this Data struct. If the Data struct does not represent a
// string, this method panics. If in doubt, check with `IsString()`
func (j Data) UnsafeStringValue() string {
	return j.data.(string)
}

// StringValue returns the golang string representation of the string
// represented by this Data struct. If the Data struct does not represent a
// string, this method returns an error.
func (j Data) StringValue() (string, error) {
	if !j.IsString() {
		return "", fmt.Errorf("This is not a string, so we can't get the StringValue of it")
	}
	return j.UnsafeStringValue(), nil
}

// IsNum returns true iff the data represented by this Data struct is a number.
func (j Data) IsNum() bool {
	return reflect.TypeOf(j.data) == reflect.TypeOf(64.4)
}

// UnsafeNumValue returns the golang float64 representation of the number represented
// by this Data struct. If the Data struct does not represent a number, this
// method panics. If in doubt, check with `IsNum()`
func (j Data) UnsafeNumValue() float64 {
	return j.data.(float64)
}

// NumValue returns the golang float64 representation of the number represented
// by this Data struct. If the Data struct does not represent a number, this
// method returns an error.
func (j Data) NumValue() (float64, error) {
	if !j.IsNum() {
		return 0, fmt.Errorf("This is not a number, so we can't get the NumValue of it")
	}
	return j.UnsafeNumValue(), nil
}

// IsBool returns true iff the data represented by this Data struct is a boolean.
func (j Data) IsBool() bool {
	return reflect.TypeOf(j.data) == reflect.TypeOf(true)
}

// UnsafeBoolValue returns the golang bool representation of the bool represented by
// this Data struct. If the Data struct does not represent a bool, this method
// panics. If in doubt, check with `IsBool()`
func (j Data) UnsafeBoolValue() bool {
	return j.data.(bool)
}

// BoolValue returns the golang bool representation of the bool represented by
// this Data struct. If the Data struct does not represent a bool, this method
// returns an error.
func (j Data) BoolValue() (bool, error) {
	if !j.IsBool() {
		return false, fmt.Errorf("This is not a bool, so we can't get the bool value of it")
	}
	return j.UnsafeBoolValue(), nil
}

// IsList returns true iff the data represented by this Data struct is a list.
func (j Data) IsList() bool {
	if j.data == nil {
		return false
	}
	return reflect.TypeOf(j.data).Kind() == reflect.TypeOf([]interface{}{}).Kind()
}

// UnsafeListValue returns a golang slice of Data structs representing the
// unstructured list represented by this Data struct.  If the Data struct does
// not represent a list, this method panics. If in doubt, check with `IsList()`
func (j Data) UnsafeListValue() (list []Data) {
	list = []Data{}
	for _, val := range j.data.([]interface{}) {
		list = append(list, Data{data: val})
	}
	return
}

// ListValue returns a golang slice of Data structs representing the
// unstructured list represented by this Data struct.  If the Data struct does
// not represent a list, this method returns an error.
func (j Data) ListValue() ([]Data, error) {
	if !j.IsList() {
		return nil, fmt.Errorf("This is not a list, so we can't get its ListValue")
	}
	return j.UnsafeListValue(), nil
}

// An ElementMatcher can be used with FindElem to find an element in an
// unstructured list.
type ElementMatcher func(Data) bool

// FindElem finds an element in a list, using a provided matcher
func (j Data) FindElem(match ElementMatcher) (Data, bool) {
	if !j.IsList() {
		return Data{}, false
	}
	for _, elem := range j.UnsafeListValue() {
		if match(elem) {
			return elem, true
		}
	}
	return Data{}, false
}

// SetElem sets the element at a given index in this Data list to the given value.
// If this Data object does not represent a list, return an error
func (j Data) SetElem(index int, value interface{}) error {
	if !j.IsList() {
		return fmt.Errorf("This is not a list, so you can't set an element of it")
	}
	j.data.([]interface{})[index] = value
	return nil
}

// IsNull returns true iff the data represented by this Data struct is null.
func (j Data) IsNull() bool {
	return j.data == nil
}

// IsOfType returns true iff the Data struct represents data of type `typ`.
// Valid values of `typ` are listed as constants above.
func (j Data) IsOfType(typ string) bool {
	switch typ {
	case DataOb:
		return j.IsOb()
	case DataString:
		return j.IsString()
	case DataList:
		return j.IsList()
	case DataNum:
		return j.IsNum()
	case DataBool:
		return j.IsBool()
	case DataNull:
		return j.IsNull()
	default:
		panic("that's not a Data type I recognise!")
	}
}
