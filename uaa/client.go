package uaa

import (
	"crypto/tls"
	"crypto/x509"
	gouaa "github.com/cloudfoundry-community/go-uaa"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	config     config.UAAConfig
	httpClient HTTPClient
	RandFunc   func() string
}

type HTTPClient interface {
	CreateClient(client gouaa.Client) (*gouaa.Client, error)
	UpdateClient(client gouaa.Client) (*gouaa.Client, error)
	DeleteClient(clientID string) (*gouaa.Client, error)
}

func New(config config.UAAConfig, trustedCert string) (*Client, error) {
	httpClient := newHTTPClient(trustedCert)

	apiClient, err := gouaa.New(
		config.URL,
		gouaa.WithClientCredentials(
			config.Authentication.ClientCredentials.ID,
			config.Authentication.ClientCredentials.Secret,
			gouaa.JSONWebToken,
		),
		gouaa.WithClient(httpClient),
	)
	return &Client{
		config:     config,
		httpClient: apiClient,
		RandFunc:   randomString,
	}, err
}

func newHTTPClient(caCert string) *http.Client {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	rootCAs.AppendCertsFromPEM([]byte(caCert))

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		},
	}
}

func (c *Client) CreateClient(clientID, name string) (map[string]string, error) {
	clientSecret := c.RandFunc()
	m := map[string]string{
		"client_id":              clientID,
		"client_secret":          clientSecret,
		"scopes":                 c.config.ClientDefinition.Scopes,
		"resource_ids":           c.config.ClientDefinition.ResourceIDs,
		"authorities":            c.config.ClientDefinition.Authorities,
		"authorized_grant_types": c.config.ClientDefinition.AuthorizedGrantTypes,
	}

	if name != "" {
		m["name"] = name
	}

	resp, err := c.httpClient.CreateClient(c.transformToClient(m))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create uaa client")
	}

	return c.transformToMap(resp, clientSecret), nil
}

func (c *Client) UpdateClient(clientID string, redirectURI string) (map[string]string, error) {
	clientSecret := c.RandFunc()
	m := map[string]string{
		"client_id":              clientID,
		"client_secret":          clientSecret,
		"scopes":                 c.config.ClientDefinition.Scopes,
		"resource_ids":           c.config.ClientDefinition.ResourceIDs,
		"authorities":            c.config.ClientDefinition.Authorities,
		"authorized_grant_types": c.config.ClientDefinition.AuthorizedGrantTypes,
		"redirect_uri":           redirectURI,
	}

	resp, err := c.httpClient.UpdateClient(c.transformToClient(m))
	if err != nil {
		return nil, errors.Wrap(err, "failed to update uaa client")
	}
	return c.transformToMap(resp, clientSecret), nil
}

func (c *Client) DeleteClient(clientID string) error {
	_, err := c.httpClient.DeleteClient(clientID)
	if err != nil {
		return errors.Wrap(err, "failed to delete client")
	}
	return err
}

func (c *Client) transformToMap(resp *gouaa.Client, secret string) map[string]string {
	return map[string]string{
		"client_id":              resp.ClientID,
		"client_secret":          secret, // client secret is not part of the response
		"name":                   resp.DisplayName,
		"scopes":                 fromSlice(resp.Scope),
		"resource_ids":           fromSlice(resp.ResourceIDs),
		"authorities":            fromSlice(resp.Authorities),
		"authorized_grant_types": fromSlice(resp.AuthorizedGrantTypes),
	}
}

func (c *Client) transformToClient(m map[string]string) gouaa.Client {
	client := gouaa.Client{
		Authorities:          toSlice(m["authorities"]),
		AuthorizedGrantTypes: toSlice(m["authorized_grant_types"]),
		ClientID:             m["client_id"],
		ClientSecret:         m["client_secret"],
		DisplayName:          m["name"],
		ResourceIDs:          toSlice(m["resource_ids"]),
		Scope:                toSlice(m["scopes"]),
		RedirectURI:          toSlice(m["redirect_uri"]),
	}

	return client
}

func fromSlice(s []string) string {
	return strings.Join(s, ",")
}

func toSlice(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func randomString() string {
	rand.Seed(time.Now().UnixNano())
	const Length = 32
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, Length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type NoopClient struct{}

func (n *NoopClient) CreateClient(_, _ string) (map[string]string, error) {
	return nil, nil
}

func (n *NoopClient) UpdateClient(_, _ string) (map[string]string, error) {
	return nil, nil
}
func (c *NoopClient) DeleteClient(clientID string) error {
	return nil
}
