// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
)

type Client struct {
	url string

	PollingInterval time.Duration
	BoshInfo        Info

	authHeaderBuilder AuthHeaderBuilder
	httpClient        NetworkDoer
}

func (c *Client) VerifyAuth(logger *log.Logger) error {
	_, err := c.GetInfo(logger)
	return err
}

//go:generate counterfeiter -o fakes/fake_auth_header_builder.go . AuthHeaderBuilder
type AuthHeaderBuilder interface {
	AddAuthHeader(request *http.Request, logger *log.Logger) error
}

//go:generate counterfeiter -o fakes/fake_network_doer.go . NetworkDoer
type NetworkDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

//go:generate counterfeiter -o fakes/fake_authenticator_builder.go . AuthenticatorBuilder
type AuthenticatorBuilder interface {
	NewAuthHeaderBuilder(boshInfo Info, disableSSLCertVerification bool) (AuthHeaderBuilder, error)
}

//go:generate counterfeiter -o fakes/fake_cert_appender.go . CertAppender
type CertAppender interface {
	AppendCertsFromPEM(pemCerts []byte) (ok bool)
}

func New(url string, disableSSLCertVerification bool, trustedCertPEM []byte, httpClient NetworkDoer, authBuilder AuthenticatorBuilder, certAppender CertAppender, logger *log.Logger) (*Client, error) {
	noAuthClient := &Client{
		url:        url,
		httpClient: httpClient,

		authHeaderBuilder: authorizationheader.NewNoAuthHeaderBuilder(),
	}
	certAppender.AppendCertsFromPEM(trustedCertPEM)

	boshInfo, err := noAuthClient.GetInfo(logger)
	if err != nil {
		return nil, fmt.Errorf("error fetching BOSH director information: %s", err)
	}

	authHeaderBuilder, err := authBuilder.NewAuthHeaderBuilder(boshInfo, disableSSLCertVerification)
	if err != nil {
		return nil, fmt.Errorf("error creating BOSH authorization header builder: %s", err)
	}

	return &Client{
			BoshInfo:          boshInfo,
			PollingInterval:   5,
			url:               url,
			httpClient:        httpClient,
			authHeaderBuilder: authHeaderBuilder},
		nil
}

type Info struct {
	Version            string
	UserAuthentication UserAuthentication `json:"user_authentication"`
}

type UserAuthentication struct {
	Options AuthenticationOptions
}

type AuthenticationOptions struct {
	URL string
}

type Deployment struct {
	Name string
}

const (
	StemcellDirectorVersionType = VersionType("stemcell")
	SemverDirectorVersionType   = VersionType("semver")
)

type VersionType string

type Version struct {
	MajorVersion int
	VersionType  VersionType
}

func NewVersion(majorVersion int, versionType VersionType) Version {
	return Version{
		MajorVersion: majorVersion,
		VersionType:  versionType,
	}
}

type DeploymentNotFoundError struct {
	error
}

type RequestError struct {
	error
}

func NewRequestError(e error) RequestError {
	return RequestError{e}
}

type unexpectedStatusError struct {
	expectedStatus int
	actualStatus   int
	responseBody   string
}

func (u unexpectedStatusError) Error() string {
	return fmt.Sprintf("expected status %d, was %d. Response Body: %s", u.expectedStatus, u.actualStatus, u.responseBody)
}

type resultExtractor func(*http.Response) error

func (c *Client) getDataCheckingForErrors(url string, expectedStatus int, result interface{}, logger *log.Logger) error {
	request, err := prepareGet(url)
	if err != nil {
		return err
	}
	return c.getResultCheckingForErrors(request, expectedStatus, decodeJson(result), logger)
}

func (c *Client) getTaskIDCheckingForErrors(url string, expectedStatus int, logger *log.Logger) (int, error) {
	request, err := prepareGet(url)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getDeploymentResultCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func (c *Client) getMultipleDataCheckingForErrors(
	url string,
	expectedStatus int,
	result interface{},
	resultReady func(),
	logger *log.Logger,
) error {
	request, err := prepareGet(url)
	if err != nil {
		return err
	}
	return c.getResultCheckingForErrors(
		request,
		expectedStatus,
		decodeMultipleJson(result, resultReady),
		logger,
	)
}

func (c *Client) postAndGetTaskIDCheckingForErrors(url string, expectedStatus int, body []byte, contentType, contextID string, logger *log.Logger) (int, error) {
	request, err := preparePost(url, body, contentType, contextID)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getResultCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func (c *Client) deleteAndGetTaskIDCheckingForErrors(url string, contextID string, expectedStatus int, logger *log.Logger) (int, error) {
	request, err := prepareDelete(url, contextID)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getDeploymentResultCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func decodeJson(result interface{}) resultExtractor {
	return func(response *http.Response) error {

		var err error
		bytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		return json.Unmarshal(bytes, result)
	}
}

func decodeMultipleJson(result interface{}, resultReady func()) resultExtractor {
	return func(response *http.Response) error {
		decoder := json.NewDecoder(response.Body)
		for decoder.More() {
			if err := decoder.Decode(&result); err != nil {
				return err
			}
			resultReady()
		}
		return nil
	}
}

func extractTaskId(taskId *int) resultExtractor {
	return func(response *http.Response) error {
		var e error
		urlParts := strings.Split(response.Header.Get("location"), "/")
		*taskId, e = strconv.Atoi(urlParts[len(urlParts)-1])
		return e
	}
}

func prepareGet(url string) (*http.Request, error) {
	return http.NewRequest("GET", url, nil)
}

func preparePost(url string, body []byte, contentType, contextID string) (*http.Request, error) {
	request, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	if contextID != "" {
		request.Header.Set("X-Bosh-Context-Id", contextID)
	}
	return request, nil
}

func prepareDelete(url, contextID string) (*http.Request, error) {
	req, err := http.NewRequest("DELETE", url, nil)

	if err != nil {
		return nil, err
	}

	if contextID != "" {
		req.Header.Set("X-Bosh-Context-Id", contextID)
	}

	return req, nil
}

func (c *Client) getDeploymentResultCheckingForErrors(request *http.Request, expectedStatus int, handler resultExtractor, logger *log.Logger) error {
	err := c.getResultCheckingForErrors(request, expectedStatus, handler, logger)
	switch err := err.(type) {
	case unexpectedStatusError:
		if err.actualStatus == http.StatusNotFound {
			return DeploymentNotFoundError{error: err}
		}
		return err
	default:
		return err
	}
}

func (c *Client) getResultCheckingForErrors(request *http.Request, expectedStatus int, handler resultExtractor, logger *log.Logger) error {
	err := c.authHeaderBuilder.AddAuthHeader(request, logger)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return NewRequestError(fmt.Errorf("error reaching bosh director: %s. Please make sure that properties.<broker-job>.bosh.url is correct and reachable.", err))
	}
	defer response.Body.Close()

	if response.StatusCode != expectedStatus {
		return unexpectedStatusErr(response, expectedStatus)
	}

	return handler(response)
}

func unexpectedStatusErr(response *http.Response, expectedStatus int) error {
	return unexpectedStatusError{
		expectedStatus: expectedStatus,
		actualStatus:   response.StatusCode,
		responseBody:   readBody(response.Body),
	}
}

func readBody(bodyReader io.ReadCloser) string {
	body, err := ioutil.ReadAll(bodyReader)
	if err == nil {
		return string(body)
	} else {
		return fmt.Sprintf("COULDN'T READ RESPONSE BODY: %s", err)
	}
}
