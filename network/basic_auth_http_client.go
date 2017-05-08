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
