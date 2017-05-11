// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/craigfurman/herottp"
)

type Client struct {
	boshURL string

	BoshPollingInterval time.Duration

	authHeaderBuilder AuthHeaderBuilder
	httpClient        HTTPClient
}

//go:generate counterfeiter -o fakes/fake_auth_header_builder.go . AuthHeaderBuilder
type AuthHeaderBuilder interface {
	Build(logger *log.Logger) (string, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func New(boshURL string, authHeaderBuilder AuthHeaderBuilder, disableSSLCertVerification bool, trustedCertPEM []byte) (*Client, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	rootCAs.AppendCertsFromPEM(trustedCertPEM)
	return &Client{
		boshURL:           boshURL,
		authHeaderBuilder: authHeaderBuilder,
		httpClient: herottp.New(herottp.Config{
			NoFollowRedirect:                  true,
			DisableTLSCertificateVerification: disableSSLCertVerification,
			RootCAs: rootCAs,
			Timeout: 30 * time.Second,
		}),
		BoshPollingInterval: 5,
	}, nil
}

type BoshInfo struct {
	Version string
}

type BoshDeployment struct {
	Name string
}

const (
	StemcellDirectorVersionType                          = BoshDirectorVersionType("stemcell")
	SemverDirectorVersionType                            = BoshDirectorVersionType("semver")
	MinimumMajorStemcellDirectorVersionForODB            = 3262
	MinimumMajorSemverDirectorVersionForLifecycleErrands = 261
)

type BoshDirectorVersionType string

type BoshDirectorVersion struct {
	majorVersion int
	versionType  BoshDirectorVersionType
}

func (v BoshDirectorVersion) SupportsODB() bool {
	if v.versionType == SemverDirectorVersionType {
		return true // First bosh director version in semver format was 260
	}

	return v.majorVersion >= MinimumMajorStemcellDirectorVersionForODB
}

func (v BoshDirectorVersion) SupportsLifecycleErrands() bool {
	if v.versionType == StemcellDirectorVersionType {
		return false // Last bosh director version in stemcell format was 259
	}

	return v.majorVersion >= MinimumMajorSemverDirectorVersionForLifecycleErrands
}

func NewBoshDirectorVersion(majorVersion int, versionType BoshDirectorVersionType) BoshDirectorVersion {
	return BoshDirectorVersion{
		majorVersion: majorVersion,
		versionType:  versionType,
	}
}

const (
	BoshTaskQueued     = "queued"
	BoshTaskProcessing = "processing"
	BoshTaskDone       = "done"
	BoshTaskError      = "error"
	BoshTaskCancelled  = "cancelled"
	BoshTaskCancelling = "cancelling"
	BoshTaskTimeout    = "timeout"
)

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

func (c *Client) getDataFromBoshCheckingForErrors(url string, expectedStatus int, result interface{}, logger *log.Logger) error {
	request, err := prepareGet(url)
	if err != nil {
		return err
	}
	return c.getResultFromBoshCheckingForErrors(request, expectedStatus, decodeJson(result), logger)
}

func (c *Client) getTaskIdFromBoshCheckingForErrors(url string, expectedStatus int, logger *log.Logger) (int, error) {
	request, err := prepareGet(url)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getDeploymentResultFromBoshCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func (c *Client) getMultipleDataFromBoshCheckingForErrors(
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
	return c.getResultFromBoshCheckingForErrors(
		request,
		expectedStatus,
		decodeMultipleJson(result, resultReady),
		logger,
	)
}

func (c *Client) postAndGetTaskIdFromBoshCheckingForErrors(url string, expectedStatus int, body []byte, contentType, contextID string, logger *log.Logger) (int, error) {
	request, err := preparePost(url, body, contentType, contextID)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getResultFromBoshCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func (c *Client) deleteAndGetTaskIdFromBoshCheckingForErrors(url string, contextID string, expectedStatus int, logger *log.Logger) (int, error) {
	request, err := prepareDelete(url, contextID)
	if err != nil {
		return 0, err
	}
	var taskId int
	err = c.getDeploymentResultFromBoshCheckingForErrors(request, expectedStatus, extractTaskId(&taskId), logger)
	return taskId, err
}

func decodeJson(result interface{}) resultExtractor {
	return func(response *http.Response) error {
		return json.NewDecoder(response.Body).Decode(result)
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

func (c *Client) getDeploymentResultFromBoshCheckingForErrors(request *http.Request, expectedStatus int, handler resultExtractor, logger *log.Logger) error {
	err := c.getResultFromBoshCheckingForErrors(request, expectedStatus, handler, logger)
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

func (c *Client) getResultFromBoshCheckingForErrors(request *http.Request, expectedStatus int, handler resultExtractor, logger *log.Logger) error {
	authHeader, err := c.authHeaderBuilder.Build(logger)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authHeader)

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
