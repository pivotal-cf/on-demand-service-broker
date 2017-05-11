// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/craigfurman/herottp"
)

type httpJsonClient struct {
	client            *herottp.Client
	AuthHeaderBuilder AuthHeaderBuilder
}

//go:generate counterfeiter -o fakes/fake_auth_header_builder.go . AuthHeaderBuilder
type AuthHeaderBuilder interface {
	Build(logger *log.Logger) (string, error)
}

func (w httpJsonClient) Get(path string, body interface{}, logger *log.Logger) error {
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	authHeader, err := w.AuthHeaderBuilder.Build(logger)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", authHeader)
	logger.Printf(fmt.Sprintf("GET %s", path))

	response, err := w.client.Do(req)
	if err != nil {
		return err
	}
	return w.readResponse(response, body)
}

func (c httpJsonClient) Delete(path string, logger *log.Logger) error {
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	authHeader, err := c.AuthHeaderBuilder.Build(logger)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", authHeader)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	logger.Printf(fmt.Sprintf("DELETE %s", path))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusAccepted, http.StatusNotFound:
		return nil
	}

	body, _ := ioutil.ReadAll(resp.Body)
	return fmt.Errorf("Unexpected reponse status %d, %q", resp.StatusCode, string(body))
}

func (w httpJsonClient) readResponse(response *http.Response, obj interface{}) error {
	defer response.Body.Close()
	rawBody, _ := ioutil.ReadAll(response.Body)

	switch response.StatusCode {
	case http.StatusOK:
		err := json.Unmarshal(rawBody, &obj)
		if err != nil {
			return NewInvalidResponseError(fmt.Sprintf("Invalid response body: %s", err))
		}

		return nil
	case http.StatusNotFound:
		return NewResourceNotFoundError(errorMessageFromRawBody(rawBody))
	case http.StatusUnauthorized:
		return NewUnauthorizedError(errorMessageFromRawBody(rawBody))
	case http.StatusForbidden:
		return NewForbiddenError(errorMessageFromRawBody(rawBody))
	default:
		return fmt.Errorf("Unexpected reponse status %d, %q", response.StatusCode, string(rawBody))
	}
}

func errorMessageFromRawBody(rawBody []byte) string {
	var body errorResponse
	err := json.Unmarshal(rawBody, &body)

	var message string
	if err != nil {
		message = string(rawBody)
	} else {
		message = body.Description
	}

	return message
}

func newWrappedHttpClient(authHeaderBuilder AuthHeaderBuilder, trustedCertPEM []byte, disableTLSCertVerification bool) (httpJsonClient, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return httpJsonClient{}, err
	}
	rootCAs.AppendCertsFromPEM(trustedCertPEM)
	config := herottp.Config{
		DisableTLSCertificateVerification: disableTLSCertVerification,
		RootCAs: rootCAs,
		Timeout: 30 * time.Second,
	}

	return httpJsonClient{
		client:            herottp.New(config),
		AuthHeaderBuilder: authHeaderBuilder,
	}, nil
}
