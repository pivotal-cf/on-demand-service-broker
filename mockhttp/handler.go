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
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

type Handler struct {
	expectedMethod         string
	expectedUrl            string
	expectedBody           string
	expectedJSONBody       string
	expectedHeadersPresent []string
	unexpectedHeaders      []string
	expectedHeaders        map[string]string

	responseRedirectToUrl string
	responseStatus        int
	responseBody          string
	delay                 time.Duration
	functions             []func()

	matchURLWithRegex bool
}

func (i *Handler) RespondsInternalServerErrorWith(body string) *Handler {
	i.responseBody = body
	i.responseStatus = http.StatusInternalServerError
	return i
}

func (i *Handler) RedirectsTo(uri string) *Handler {
	i.responseStatus = http.StatusFound
	i.responseRedirectToUrl = uri
	return i
}

func (i *Handler) RespondsNotFoundWith(body string) *Handler {
	i.responseBody = body
	i.responseStatus = http.StatusNotFound
	return i
}

func (i *Handler) RespondsUnauthorizedWith(body string) *Handler {
	i.responseBody = body
	i.responseStatus = http.StatusUnauthorized
	return i
}

func (i *Handler) RespondsForbiddenWith(body string) *Handler {
	i.responseBody = body
	i.responseStatus = http.StatusForbidden
	return i
}

func (i *Handler) RespondsOKWithJSON(obj interface{}) *Handler {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	i.RespondsOKWith(string(data))
	return i
}

func (i *Handler) RespondsOKWith(body string) *Handler {
	i.responseStatus = http.StatusOK
	i.responseBody = body
	return i
}

func (i *Handler) RespondsAcceptedWith(body string) *Handler {
	i.responseStatus = http.StatusAccepted
	i.responseBody = body
	return i
}

func (i *Handler) RespondsCreated() *Handler {
	i.responseStatus = http.StatusCreated
	i.responseBody = ""
	return i
}

func (i *Handler) RespondsNoContent() *Handler {
	i.responseStatus = http.StatusNoContent
	i.responseBody = ""
	return i
}

func (i *Handler) DelayResponse(delay time.Duration) *Handler {
	i.delay = delay
	return i
}

func (i *Handler) WithBody(body string) *Handler {
	i.expectedBody = body
	return i
}

func (i *Handler) WithJSONBody(body string) *Handler {
	i.expectedJSONBody = body
	return i
}

func (i *Handler) WithContentType(contentType string) *Handler {
	i.expectedHeaders["Content-Type"] = contentType
	return i
}

func (i *Handler) WithAuthorizationHeader(auth string) *Handler {
	i.expectedHeaders["Authorization"] = auth
	return i
}

func (i *Handler) WithHeaderPresent(header string) *Handler {
	i.expectedHeadersPresent = append(i.expectedHeadersPresent, header)
	return i
}

func (i *Handler) WithoutHeader(header string) *Handler {
	i.unexpectedHeaders = append(i.unexpectedHeaders, header)
	return i
}

func (i *Handler) WithHeader(header, value string) *Handler {
	i.expectedHeaders[header] = value
	return i
}

func (i *Handler) WithRegexURLMatcher() *Handler {
	i.matchURLWithRegex = true
	return i
}

func (i *Handler) RunsFunction(expectedFunction func()) *Handler {
	i.functions = append(i.functions, expectedFunction)
	return i
}

func NewMockedHttpRequest(method, url string) *Handler {
	return &Handler{
		expectedMethod:         method,
		expectedUrl:            url,
		expectedHeaders:        make(map[string]string),
		expectedHeadersPresent: []string{},
		unexpectedHeaders:      []string{},
		matchURLWithRegex:      false,
	}
}

func (i *Handler) Verify(req *http.Request, s *Server) {
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
	if i.expectedJSONBody != "" {
		rawBody, err := ioutil.ReadAll(req.Body)
		Expect(err).NotTo(HaveOccurred())

		Expect(rawBody).To(MatchJSON(i.expectedJSONBody), "Expected JSON body:\n\t%+v\nto match json to:\n\t%+v\n", rawBody, i.expectedJSONBody)
	}
}

func (i *Handler) Respond(writer http.ResponseWriter, logger *log.Logger) {
	for _, function := range i.functions {
		function()
	}
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

func (i *Handler) For(comment string) *Handler {
	return i
}

func (i *Handler) Url() string {
	return i.expectedMethod + " " + i.expectedUrl
}

func unexpectedRequestDescription(req *http.Request, s *Server) string {
	received := req.Method + " " + req.URL.String()
	completedMocks := strings.Join(s.completedMocks(), "\n")
	pendingMocks := strings.Join(s.pendingMocks(), "\n")
	return fmt.Sprintf("Unexpected request:\n\t%s\nReceived by:\n\t%s\nCompleted:\n%s\nPending:\n%s\n", received, s.name, completedMocks, pendingMocks)
}
