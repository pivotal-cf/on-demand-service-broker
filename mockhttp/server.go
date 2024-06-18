// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mockhttp

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

type Server struct {
	name, expectedAuthorizationHeader string
	excludedAuthPaths                 map[string]bool

	*httptest.Server
	*sync.Mutex

	mockHandlers   []MockedResponseBuilder
	currentHandler int

	logger *log.Logger
}

func StartServer(name string) *Server {
	s := &Server{
		name:              name,
		Mutex:             new(sync.Mutex),
		excludedAuthPaths: make(map[string]bool),
	}
	s.Server = httptest.NewServer(s)

	s.logger = log.New(GinkgoWriter, "["+name+"] ", log.LstdFlags)
	return s
}

func StartTLSServer(name, certPath, keyPath string) *Server {
	s := &Server{
		name:              name,
		Mutex:             new(sync.Mutex),
		excludedAuthPaths: make(map[string]bool),
	}
	cer, err := tls.LoadX509KeyPair(certPath, keyPath)
	Expect(err).NotTo(HaveOccurred())
	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	sslServer := httptest.NewUnstartedServer(s)
	sslServer.TLS = config
	sslServer.Config.ErrorLog = loggerfactory.New(GinkgoWriter, "server", loggerfactory.Flags).New()
	s.Server = sslServer
	s.Server.StartTLS()

	s.logger = log.New(GinkgoWriter, "["+name+"] ", log.LstdFlags)
	return s
}

func (s *Server) ExpectedAuthorizationHeader(header string) {
	s.expectedAuthorizationHeader = header
}

func (s *Server) ExpectedBasicAuth(username, password string) {
	s.ExpectedAuthorizationHeader(basicAuth(username, password))
}

func (s *Server) ExcludeAuthorizationCheck(path string) {
	s.excludedAuthPaths[path] = true
}

func (s *Server) checkAuthHeaderForPath(path string) bool {
	return !s.excludedAuthPaths[path]
}

func (s *Server) verifyCommonServerExpectations(r *http.Request) {
	if s.expectedAuthorizationHeader != "" && s.checkAuthHeaderForPath(r.URL.Path) {
		Expect(r.Header.Get("Authorization")).To(Equal(s.expectedAuthorizationHeader), "Expected 'Authorization' header to be equal to:\n    %+v\n", s.expectedAuthorizationHeader)
	}
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	s.Lock()
	defer s.Unlock()
	defer GinkgoRecover()

	if s.currentHandler >= len(s.mockHandlers) {
		received := req.Method + " " + req.URL.String()
		completedMocks := strings.Join(s.completedMocks(), "\n")
		pendingMocks := strings.Join(s.pendingMocks(), "\n")
		Fail(fmt.Sprintf("Unmocked request:\n\t%s\nReceived by:\n\t%s\nCompleted:\n%s\nPending:\n%s\n", received, s.name, completedMocks, pendingMocks))
	}

	s.logger.Printf("%s %s\n", req.Method, req.URL.String())
	currentHandlerIndex := s.currentHandler
	s.currentHandler += 1
	s.mockHandlers[currentHandlerIndex].Verify(req, s)
	s.mockHandlers[currentHandlerIndex].Respond(writer, s.logger)
}

func (s *Server) completedMocks() []string {
	var completedMocks []string
	for i := 0; i < s.currentHandler; i++ {
		completedMocks = append(completedMocks, "\t"+s.mockHandlers[i].Url())
	}
	return completedMocks
}

func (s *Server) pendingMocks() []string {
	var pendingMocks []string
	for i := s.currentHandler; i < len(s.mockHandlers); i++ {
		pendingMocks = append(pendingMocks, "\t"+s.mockHandlers[i].Url())
	}
	return pendingMocks
}

func (s *Server) VerifyAndMock(mockedResponses ...MockedResponseBuilder) {
	s.Lock()
	defer s.Unlock()
	s.VerifyMocks()

	s.currentHandler = 0
	s.mockHandlers = mockedResponses
}

func (s *Server) AppendMocks(mockedResponses ...MockedResponseBuilder) {
	s.Lock()
	defer s.Unlock()

	s.mockHandlers = append(s.mockHandlers, mockedResponses...)
}

func (s *Server) VerifyMocks() {
	if len(s.mockHandlers) != s.currentHandler {
		completedMocks := strings.Join(s.completedMocks(), "\n")
		pendingMocks := strings.Join(s.pendingMocks(), "\n")
		Fail(fmt.Sprintf("Uninvoked mocks for:\n\t%s\nCompleted:\n%s\nPending:\n%s\n", s.name, completedMocks, pendingMocks))
	}
}

type MockedResponseBuilder interface {
	Verify(req *http.Request, d *Server)
	Respond(writer http.ResponseWriter, logger *log.Logger)
	Url() string
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}
