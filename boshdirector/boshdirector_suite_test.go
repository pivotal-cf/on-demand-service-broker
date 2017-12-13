// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"

	"bytes"

	"reflect"

	"net/url"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var (
	c *boshdirector.Client

	fakeHTTPClient           *fakes.FakeNetworkDoer
	fakeAuthenticatorBuilder *fakes.FakeAuthenticatorBuilder
	authHeaderBuilder        *fakes.FakeAuthHeaderBuilder
	fakeCertAppender         *fakes.FakeCertAppender
	fakeDirector             *fakes.FakeDirector
	fakeDirectorFactory      *fakes.FakeDirectorFactory
	fakeUAAFactory           *fakes.FakeUAAFactory
	fakeUAA                  *fakes.FakeUAA
	logger                   *log.Logger
	boshAuthConfig           config.Authentication
)

var _ = BeforeEach(func() {
	fakeHTTPClient = new(fakes.FakeNetworkDoer)
	fakeAuthenticatorBuilder = new(fakes.FakeAuthenticatorBuilder)
	authHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
	fakeAuthenticatorBuilder.NewAuthHeaderBuilderReturns(authHeaderBuilder, nil)
	fakeCertAppender = new(fakes.FakeCertAppender)
	fakeDirectorFactory = new(fakes.FakeDirectorFactory)
	fakeUAAFactory = new(fakes.FakeUAAFactory)
	fakeUAA = new(fakes.FakeUAA)
	fakeDirector = new(fakes.FakeDirector)
	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			ClientCredentials: config.ClientCredentials{
				ID:     "bosh-user",
				Secret: "bosh-secret",
			},
		},
	}
	logger = log.New(GinkgoWriter, "[boshdirector unit test]", log.LstdFlags)

	fakeHTTPClient.DoStub = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(`{}`)),
			StatusCode: http.StatusOK,
		}, nil
	}

	fakeDirectorFactory.NewReturns(fakeDirector, nil)
	fakeUAAFactory.NewReturns(fakeUAA, nil)
	fakeDirector.InfoReturns(boshdir.Info{
		Version: "1.3262.0.0 (00000000)",
		Auth: boshdir.UserAuthentication{
			Type: "uaa",
			Options: map[string]interface{}{
				"url": "https://this-is-the-uaa-url.example.com",
			},
		},
	}, nil)
})

var _ = JustBeforeEach(func() {
	var certPEM []byte

	var err error

	c, err = boshdirector.New(
		"https://director.example.com",
		false,
		certPEM,
		fakeHTTPClient,
		fakeAuthenticatorBuilder,
		fakeCertAppender,
		fakeDirectorFactory,
		fakeUAAFactory,
		boshAuthConfig,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
})

func responseWithEmptyBodyAndStatus(statusCode int) *http.Response {
	return &http.Response{
		Body:       ioutil.NopCloser(strings.NewReader("")),
		StatusCode: statusCode,
	}
}

func responseWithRawManifest(rawManifest []byte) *http.Response {
	data := map[string]string{"manifest": string(rawManifest)}
	return responseOKWithJSON(data)
}

func responseOKWithTaskOutput(taskOutputs []boshdirector.BoshVMsOutput) *http.Response {
	body := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(body)

	for _, line := range taskOutputs {
		err := encoder.Encode(line)
		if err != nil {
			panic("Error while encoding task output")
		}
	}
	return responseOKWithRawBody(body.Bytes())
}

func responseOKWithJSON(data interface{}) *http.Response {
	bytes, err := json.Marshal(data)
	Expect(err).NotTo(HaveOccurred())

	return responseOKWithRawBody(bytes)
}

func responseOKWithRawBody(body []byte) *http.Response {
	resp := responseWithEmptyBodyAndStatus(http.StatusOK)
	resp.Body = ioutil.NopCloser(strings.NewReader(string(body)))

	return resp
}

func responseWithRedirectToTaskID(taskID int) *http.Response {
	resp := responseWithEmptyBodyAndStatus(http.StatusFound)
	resp.Header = http.Header{}
	resp.Header.Set("Location", taskURL(taskID))
	return resp
}

func taskURL(taskID int) string {
	return fmt.Sprintf("/tasks/%d", taskID)
}

func TestBoshDirector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh Director Suite")
}

type receivedHttpRequest struct {
	Host   string
	Path   string
	Method string
	Header http.Header
	Query  url.Values
}

type receivedHttpRequestMatcher struct {
	Expected receivedHttpRequest
	Index    int
}

func (matcher *receivedHttpRequestMatcher) Match(actual interface{}) (success bool, err error) {
	fakeHTTPClient, ok := actual.(*fakes.FakeNetworkDoer)
	if !ok {
		return false, fmt.Errorf("HaveReceivedHttpRequest matcher requires actual value to be castable to type fakes.FakeNetworkDoer")
	}

	callCount := fakeHTTPClient.DoCallCount()
	if callCount <= matcher.Index {
		return false, fmt.Errorf("expected number of http calls to be > %d", matcher.Index)
	}

	argsForCall := fakeHTTPClient.DoArgsForCall(matcher.Index)

	if matcher.Expected.Host != "" {
		if argsForCall.Host != matcher.Expected.Host {
			return false, fmt.Errorf("expected host to equal %s, actual was %s", matcher.Expected.Host, argsForCall.Host)
		}
	}

	if matcher.Expected.Method != "" {
		if argsForCall.Method != matcher.Expected.Method {
			return false, fmt.Errorf("expected method to equal %s, actual was %s", matcher.Expected.Method, argsForCall.Method)
		}
	}

	if matcher.Expected.Path != "" {
		if argsForCall.URL.Path != matcher.Expected.Path {
			return false, fmt.Errorf("expected path to equal %s, actual was %s", matcher.Expected.Path, argsForCall.URL.Path)
		}
	}

	if len(matcher.Expected.Header) > 0 {
		for key, value := range matcher.Expected.Header {
			actualValues := argsForCall.Header[key]
			if len(actualValues) == 0 {
				return false, fmt.Errorf("expected header %s to be set", key)
			}
			if !reflect.DeepEqual(actualValues, value) {
				return false, fmt.Errorf("expected values for header %s to equal %v, actual was %v", key, value, actualValues)
			}
		}
	}

	if len(matcher.Expected.Query) > 0 {
		actualQueryParams := argsForCall.URL.Query()
		if !reflect.DeepEqual(actualQueryParams, matcher.Expected.Query) {
			return false, fmt.Errorf("expected query parameters to equal %v, actual was %v", matcher.Expected.Query, actualQueryParams)
		}
	}

	return true, nil
}

func (matcher *receivedHttpRequestMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to match request", matcher.Expected)
}

func (matcher *receivedHttpRequestMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to match request", matcher.Expected)
}

func HaveReceivedHttpRequestAtIndex(expected receivedHttpRequest, index int) types.GomegaMatcher {
	return &receivedHttpRequestMatcher{
		Expected: expected,
		Index:    index,
	}
}
