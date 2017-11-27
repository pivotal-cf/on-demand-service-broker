package gunstructured_test

import (
	"github.com/totherme/unstructured/gunstructured"

	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/totherme/unstructured"
)

var _ = Describe("HavePointerMatcher", func() {
	var json unstructured.Data
	BeforeEach(func() {
		rawjson := `{"name": "fred",
							"othernames": [
								"alice",
								"bob",
								"ezekiel"
							],
							"life": 42,
							"things": {
								"more": "things"
							},
							"beauty": true,
							"not": null
						}`
		var err error
		json, err = unstructured.ParseJSON(rawjson)
		Expect(err).NotTo(HaveOccurred())

	})
	Describe("Match", func() {
		Context("When we give it a Data object", func() {
			DescribeTable("the matcher matches iff HasPointer returns true", func(p string) {

				var matcher types.GomegaMatcher = gunstructured.HaveJSONPointer(p)
				hasp, err := json.HasPointer(p)
				Expect(err).NotTo(HaveOccurred())
				Expect(matcher.Match(json)).To(Equal(hasp))
			},
				Entry("a string pointer", "/name"),
				Entry("a number pointer", "/life"),
				Entry("a list pointer", "/othernames"),
				Entry("a boolean pointer", "/beauty"),
				Entry("an object pointer", "/things"),
				Entry("a pointer to null", "/not"),
				Entry("an absent pointer", "/badgers"),
				Entry("a long pointer", "/things/more"),
			)
		})

		Context("when we give it a non-json object", func() {
			It("returns a helpful error message", func() {
				matcher := gunstructured.HaveJSONPointer("/perfectly/valid")
				_, err := matcher.Match(`{"you":"might almost think this would work"}`)
				Expect(err).To(MatchError(ContainSubstring("not a Data object. Have you done unstructured.Parse[JSON|YAML](...)?")))
			})
		})
		Context("when we give it an invalid pointer", func() {
			It("returns a helpful error message", func() {
				matcher := gunstructured.HaveJSONPointer("not/a/valid/pointer")
				_, err := matcher.Match(json)
				Expect(err).To(MatchError(ContainSubstring("JSON pointer must be empty or start with a \"/\"")))
			})
		})
	})
	Describe("FailureMessage", func() {
		It("should tell us what pointer we expected to find", func() {
			Expect(gunstructured.HaveJSONPointer("/my/pointer").FailureMessage("actual-object")).
				To(ContainSubstring("expected 'actual-object' to be a unstructured.Data object with pointer '/my/pointer'"))
		})
		Context("when the input has a long string representation", func() {
			It("truncates that representation", func() {
				Expect(len(gunstructured.HaveJSONPointer("/pointer").FailureMessage(json))).To(BeNumerically("<", 125))
				Expect(gunstructured.HaveJSONPointer("/pointer").FailureMessage(json)).
					To(ContainSubstring("..."))
				Expect(gunstructured.HaveJSONPointer("/pointer").FailureMessage(json)).
					To(ContainSubstring("{data:map"))
			})
		})

		Context("when the input's string representation is exactly as large as we're willing to print", func() {
			It("prints it all, without elipses", func() {
				stringOfLength50 := strings.Repeat("a", 50)
				failureMessage := gunstructured.HaveJSONPointer("/pointer").FailureMessage(stringOfLength50)
				Expect(failureMessage).To(ContainSubstring(fmt.Sprintf("'%s'", stringOfLength50)))
				Expect(failureMessage).NotTo(ContainSubstring("..."))
			})
		})
	})

	Describe("NegatedFailureMessage", func() {
		var (
			shortJson      unstructured.Data
			jsonOfLength50 unstructured.Data
		)

		BeforeEach(func() {
			var err error
			shortJson, err = unstructured.ParseJSON(`{"key":"val"}`)
			Expect(err).NotTo(HaveOccurred())
			jsonOfLength50, err = unstructured.ParseJSON(fmt.Sprintf(`{"key":"%s"}`, strings.Repeat("a", 34)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should tell us what pointer we expected not to find", func() {
			Expect(gunstructured.HaveJSONPointer("/key").NegatedFailureMessage(shortJson)).
				To(ContainSubstring("expected '{data:map[key:val]}' not to contain the pointer '/key'"))
		})

		Context("when the input has a long string representation", func() {
			It("truncates that representation", func() {
				Expect(len(gunstructured.HaveJSONPointer("/beauty").NegatedFailureMessage(json))).To(BeNumerically("<", 102))
				Expect(gunstructured.HaveJSONPointer("/beauty").NegatedFailureMessage(json)).
					To(ContainSubstring("..."))
				Expect(gunstructured.HaveJSONPointer("/beauty").NegatedFailureMessage(json)).
					To(ContainSubstring("{data:map"))
			})
		})

		Context("when the input is exactly as large as we're willing to print", func() {
			It("prints it all, without elipses", func() {
				failureMessage := gunstructured.HaveJSONPointer("/key").NegatedFailureMessage(jsonOfLength50)
				Expect(failureMessage).To(ContainSubstring(fmt.Sprintf("'%+v'", jsonOfLength50)))
				Expect(failureMessage).NotTo(ContainSubstring("..."))
			})
		})
	})
})
