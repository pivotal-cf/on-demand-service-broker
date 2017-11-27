package main

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	"github.com/totherme/unstructured"
)

func main() {
	myYaml := `
top-level-list:
- "this first element is a string - perhaps containing metadata"
- name: first real element
  type: element-type-1
  payload: [1,2,3,4]
- name: second real element
  type: element-type-2
  payload:
    some: embedded structure
`

	myData, err := unstructured.ParseYAML(myYaml)
	if err != nil {
		panic("Couldn't parse my own yaml")
	}

	myPayloadData, err := myData.GetByPointer("/top-level-list/2/payload/some")
	if err != nil {
		panic("Couldn't address into my own yaml")
	}

	myPayloadValue, err := myPayloadData.StringValue()
	if err != nil {
		panic("I really expected myPayloadData to represent a string...")
	}
	fmt.Println(myPayloadValue)

	myPayloadMap, err := myData.GetByPointer("/top-level-list/2/payload")
	if err != nil {
		panic("Couldn't address into my own yaml")
	}

	err = myPayloadMap.SetField("additional-key", []string{"some", "arbitrary", "data"})
	if err != nil {
		panic("I relly expected myPayloadMap to be an Object that I could write fields into")
	}

	outputYaml, err := yaml.Marshal(myData.RawValue())
	if err != nil {
		panic("myData should definitely still be serializable")
	}
	fmt.Println(string(outputYaml))
}
