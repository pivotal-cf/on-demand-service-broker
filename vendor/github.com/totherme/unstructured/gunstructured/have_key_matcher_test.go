package gunstructured_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/totherme/unstructured"
	"github.com/totherme/unstructured/gunstructured"
)

var _ = Describe("HaveJSONKeyMatcher", func() {
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
			DescribeTable("the matcher matches iff HasKey returns true", func(key string) {

				var matcher types.GomegaMatcher = gunstructured.HaveJSONKey(key)
				Expect(matcher.Match(json)).To(Equal(json.HasKey(key)))
			},
				Entry("a string key", "name"),
				Entry("a number key", "life"),
				Entry("a list key", "othernames"),
				Entry("a boolean key", "beauty"),
				Entry("an object key", "things"),
				Entry("a null key", "not"),
				Entry("an absent key", "badgers"),
			)
		})
		Context("when we give it a non-json object", func() {
			It("returns a helpful error message", func() {
				matcher := gunstructured.HaveJSONKey("key")
				_, err := matcher.Match(`{"you":"might almost think this would work"}`)
				Expect(err).To(MatchError(ContainSubstring("not a Data object. Have you done unstructured.Parse[JSON|YAML](...)?")))
			})
		})
	})

	Describe("FailureMessage", func() {
		It("should tell us what key we expected to find", func() {
			Expect(gunstructured.HaveJSONKey("my-key").FailureMessage("actual-object")).
				To(ContainSubstring("expected 'actual-object' to be an unstructured.Data object with key 'my-key'"))
			Expect(gunstructured.HaveJSONKey("my-key").FailureMessage(42)).
				To(ContainSubstring("expected '42' to be an unstructured.Data object with key 'my-key'"))
		})

		Context("when the input has a long string representation", func() {
			It("truncates that representation", func() {
				Expect(len(gunstructured.HaveJSONKey("absent-key").FailureMessage(json))).To(BeNumerically("<", 125))
				Expect(gunstructured.HaveJSONKey("absent-key").FailureMessage(json)).
					To(ContainSubstring("..."))
				Expect(gunstructured.HaveJSONKey("absent-key").FailureMessage(json)).
					To(ContainSubstring("{data:map"))
			})
		})

		Context("when the input's string representation is exactly as large as we're willing to print", func() {
			It("prints it all, without elipses", func() {
				stringOfLength50 := strings.Repeat("a", 50)
				failureMessage := gunstructured.HaveJSONKey("absent-key").FailureMessage(stringOfLength50)
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

		It("should tell us what key we expected not to find", func() {
			Expect(gunstructured.HaveJSONKey("key").NegatedFailureMessage(shortJson)).
				To(ContainSubstring("expected '{data:map[key:val]}' not to contain the key 'key'"))
		})

		Context("when the input has a long string representation", func() {
			It("truncates that representation", func() {
				Expect(len(gunstructured.HaveJSONKey("beauty").NegatedFailureMessage(json))).To(BeNumerically("<", 100))
				Expect(gunstructured.HaveJSONKey("beauty").NegatedFailureMessage(json)).
					To(ContainSubstring("..."))
				Expect(gunstructured.HaveJSONKey("beauty").NegatedFailureMessage(json)).
					To(ContainSubstring("{data:map"))
			})
		})

		Context("when the input is exactly as large as we're willing to print", func() {
			It("prints it all, without elipses", func() {
				failureMessage := gunstructured.HaveJSONKey("key").NegatedFailureMessage(jsonOfLength50)
				Expect(failureMessage).To(ContainSubstring(fmt.Sprintf("'%+v'", jsonOfLength50)))
				Expect(failureMessage).NotTo(ContainSubstring("..."))
			})
		})
	})
})
