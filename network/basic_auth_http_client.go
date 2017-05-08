// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package network

import (
	"net/http"
	"net/url"
	"strings"
)

type BasicAuthHTTPClient struct {
	doer                        Doer
	username, password, baseURL string
}

//go:generate counterfeiter -o fakes/fake_doer.go . Doer
type Doer interface {
	Do(request *http.Request) (*http.Response, error)
}

func NewBasicAuthHTTPClient(doer Doer, username, password, baseURL string) *BasicAuthHTTPClient {
	return &BasicAuthHTTPClient{
		doer:     doer,
		username: username,
		password: password,
		baseURL:  baseURL,
	}
}

func (b *BasicAuthHTTPClient) Get(path string, query map[string]string) (*http.Response, error) {
	u, err := b.buildURL(path, query)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	return b.do(request)
}

func (b *BasicAuthHTTPClient) Patch(path string) (*http.Response, error) {
	u, err := b.buildURL(path, nil)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("PATCH", u, nil)
	if err != nil {
		return nil, err
	}

	return b.do(request)
}

func (b *BasicAuthHTTPClient) buildURL(path string, query map[string]string) (string, error) {
	base := b.baseURL
	if strings.HasSuffix(b.baseURL, "/") {
		base = strings.TrimRight(b.baseURL, "/")
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	url := base + path
	if query != nil {
		return appendQuery(url, query), nil
	}
	return url, nil
}

func (b *BasicAuthHTTPClient) do(request *http.Request) (*http.Response, error) {
	request.SetBasicAuth(b.username, b.password)
	return b.doer.Do(request)
}

func appendQuery(u string, query map[string]string) string {
	values := url.Values{}
	for param, value := range query {
		values.Set(param, value)
	}
	return u + "?" + values.Encode()
}
