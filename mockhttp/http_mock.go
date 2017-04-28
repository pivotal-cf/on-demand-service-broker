// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"

	. "github.com/onsi/gomega"
	"time"
)

type MockHttp struct {
	expectedMethod         string
	expectedUrl            string
	expectedBody           string
	expectedJsonBody       map[string]interface{}
	expectedHeadersPresent []string
	unexpectedHeaders      []string
	expectedHeaders        map[string]string

	responseRedirectToUrl string
	responseStatus        int
	responseBody          string
	delay                 time.Duration

	matchURLWithRegex bool
}

func (i *MockHttp) Fails(message string) *MockHttp {
	i.responseBody = message
	i.responseStatus = http.StatusInternalServerError
	return i
}

func (i *MockHttp) RedirectsTo(uri string) *MockHttp {
	i.responseStatus = http.StatusFound
	i.responseRedirectToUrl = uri
	return i
}

func (i *MockHttp) NotFound() *MockHttp {
	i.responseBody = ""
	i.responseStatus = http.StatusNotFound
	return i
}

func (i *MockHttp) NotFoundWithBody(body string) *MockHttp {
	i.responseBody = body
	i.responseStatus = http.StatusNotFound
	return i
}

func (i *MockHttp) RespondsWithUnauthorized(body string) *MockHttp {
	i.responseBody = body
	i.responseStatus = http.StatusUnauthorized
	return i
}

func (i *MockHttp) RespondsWithForbidden(body string) *MockHttp {
	i.responseBody = body
	i.responseStatus = http.StatusForbidden
	return i
}

func (i *MockHttp) RespondsWithJson(obj interface{}) *MockHttp {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	i.RespondsWith(string(data))
	return i
}

func (i *MockHttp) RespondsAcceptedWith(body string) *MockHttp {
	i.responseStatus = http.StatusAccepted
	i.responseBody = body
	return i
}

func (i *MockHttp) RespondsWith(body string) *MockHttp {
	i.responseStatus = http.StatusOK
	i.responseBody = body
	return i
}

func (i *MockHttp) RespondsAccepted() *MockHttp {
	i.responseStatus = http.StatusAccepted
	i.responseBody = ""
	return i
}

func (i *MockHttp) RespondsNoContent() *MockHttp {
	i.responseStatus = http.StatusNoContent
	i.responseBody = ""
	return i
}

func (i *MockHttp) DelayResponse(delay time.Duration) *MockHttp {
	i.delay = delay
	return i
}

func (i *MockHttp) WithBody(body string) *MockHttp {
	i.expectedBody = body
	return i
}

func (i *MockHttp) WithJsonBody(body map[string]interface{}) *MockHttp {
	i.expectedJsonBody = body
	return i
}

func (i *MockHttp) WithContentType(contentType string) *MockHttp {
	i.expectedHeaders["Content-Type"] = contentType
	return i
}

func (i *MockHttp) WithAuthorizationHeader(auth string) *MockHttp {
	i.expectedHeaders["Authorization"] = auth
	return i
}

func (i *MockHttp) WithHeaderPresent(header string) *MockHttp {
	i.expectedHeadersPresent = append(i.expectedHeadersPresent, header)
	return i
}

func (i *MockHttp) WithoutHeader(header string) *MockHttp {
	i.unexpectedHeaders = append(i.unexpectedHeaders, header)
	return i
}

func (i *MockHttp) WithHeader(header, value string) *MockHttp {
	i.expectedHeaders[header] = value
	return i
}

func (i *MockHttp) WithRegexURLMatcher() *MockHttp {
	i.matchURLWithRegex = true
	return i
}

func NewMockedHttpRequest(method, url string) *MockHttp {
	return &MockHttp{
		expectedMethod:         method,
		expectedUrl:            url,
		expectedHeaders:        make(map[string]string),
		expectedHeadersPresent: []string{},
		unexpectedHeaders:      []string{},
		matchURLWithRegex:      false,
	}
}

func (i *MockHttp) Verify(req *http.Request, s *Server) {
	if i.matchURLWithRegex == true {
		Expect(req.Method+" "+req.URL.String()).To(MatchRegexp(i.expectedMethod+" "+i.expectedUrl), unexpectedRequestDescription(req, s))
	} else {
		Expect(req.Method+" "+req.URL.String()).To(Equal(i.expectedMethod+" "+i.expectedUrl), unexpectedRequestDescription(req, s))
	}

	s.verifyCommonServerExpectations(req)

	for _, header := range i.expectedHeadersPresent {
		Expect(req.Header.Get(header)).NotTo(BeEmpty(), "Expected header:\n\t%s\nto be present.\n", header)
	}
	for _, header := range i.unexpectedHeaders {
		Expect(req.Header.Get(header)).To(BeEmpty(), "Expected header:\n\t%s\nnot to be present.\n", header)
	}

	for header, value := range i.expectedHeaders {
		Expect(req.Header.Get(header)).To(Equal(value), "Expected header:\n\t%s\nto be equal to:\n\t%s\n", header, value)
	}
	if i.expectedBody != "" {
		rawBody, err := ioutil.ReadAll(req.Body)
		Expect(err).NotTo(HaveOccurred(), "Expected body.\n")
		Expect(string(rawBody)).To(Equal(i.expectedBody), "Expected body:\n\t%s\nto be equal to:\n\t%s\n", i.expectedBody)
	}
	if i.expectedJsonBody != nil {
		rawBody, err := ioutil.ReadAll(req.Body)
		Expect(err).NotTo(HaveOccurred(), "Expected JSON body.\n")

		var jsonBody map[string]interface{}
		err = json.Unmarshal(rawBody, &jsonBody)
		Expect(err).NotTo(HaveOccurred(), "Expected JSON body.\n")

		equalBodies := reflect.DeepEqual(jsonBody, i.expectedJsonBody)
		Expect(equalBodies).To(BeTrue(), "Expected JSON body:\n\t%+v\nto be deep equal to:\n\t%+v\n", jsonBody, i.expectedJsonBody)
	}
}

func (i *MockHttp) Respond(writer http.ResponseWriter, logger *log.Logger) {
	if i.delay != 0 {
		time.Sleep(i.delay)
	}
	if len(i.responseRedirectToUrl) != 0 {
		logger.Printf("Redirecting to %s\n", i.responseRedirectToUrl)
		writer.Header().Set("Location", i.responseRedirectToUrl)
	}
	logger.Printf("Responding with code(%d)\n%s\n", i.responseStatus, i.responseBody)
	writer.WriteHeader(i.responseStatus)
	io.WriteString(writer, i.responseBody)
}

func (i *MockHttp) For(comment string) *MockHttp {
	return i
}

func (i *MockHttp) Url() string {
	return i.expectedMethod + " " + i.expectedUrl
}

func unexpectedRequestDescription(req *http.Request, s *Server) string {
	received := req.Method + " " + req.URL.String()
	completedMocks := strings.Join(s.completedMocks(), "\n")
	pendingMocks := strings.Join(s.pendingMocks(), "\n")
	return fmt.Sprintf("Unexpected request:\n\t%s\nReceived by:\n\t%s\nCompleted:\n%s\nPending:\n%s\n", received, s.name, completedMocks, pendingMocks)
}
