// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package authorizationheader

import (
	"crypto/x509"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/craigfurman/herottp"
)

type UserTokenAuthHeaderBuilder struct {
	uaaURL            string
	clientID          string
	clientSecret      string
	username          string
	password          string
	httpClient        HTTPClient
	cachedToken       string
	cachedTokenExpiry time.Time
	tokenLock         *sync.Mutex
}

func NewUserTokenAuthHeaderBuilder(
	uaaURL,
	clientID,
	clientSecret,
	username,
	password string,
	disableSSLCertVerification bool,
	trustedCertPEM []byte,
) (*UserTokenAuthHeaderBuilder, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	rootCAs.AppendCertsFromPEM(trustedCertPEM)
	return &UserTokenAuthHeaderBuilder{
		uaaURL:       uaaURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     username,
		password:     password,
		httpClient: herottp.New(herottp.Config{
			DisableTLSCertificateVerification: disableSSLCertVerification,
			RootCAs: rootCAs,
			Timeout: 30 * time.Second,
		}),
		tokenLock: new(sync.Mutex),
	}, nil
}

func (hb *UserTokenAuthHeaderBuilder) Build(logger *log.Logger) (string, error) {
	hb.tokenLock.Lock()
	defer hb.tokenLock.Unlock()

	var err error
	hb.cachedToken, hb.cachedTokenExpiry, err = getValidToken(hb.cachedToken, hb.cachedTokenExpiry, logger, hb.obtainToken)
	if err != nil {
		return "", err
	}

	return bearerToken(hb.cachedToken), nil
}

func (hb *UserTokenAuthHeaderBuilder) obtainToken(logger *log.Logger) (string, int, error) {
	requestBody := fmt.Sprintf("grant_type=password&username=%s&password=%s", hb.username, hb.password)

	request, err := buildTokenRequest(requestBody, hb.uaaURL, hb.clientID, hb.clientSecret)
	if err != nil {
		return "", 0, err
	}

	return doObtainTokenRequest(hb.httpClient, logger, request)
}
