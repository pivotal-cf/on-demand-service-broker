package uaa

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	gouaa "github.com/cloudfoundry-community/go-uaa"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"
)

const (
	placeholderRedirectURI = "https://placeholder.example.com"
	odbSpaceGUID           = "ODB_SPACE_GUID"
)

type Client struct {
	config    config.UAAConfig
	apiClient APIClient
	RandFunc  func() string
}

type APIClient interface {
	CreateClient(client gouaa.Client) (*gouaa.Client, error)
	UpdateClient(client gouaa.Client) (*gouaa.Client, error)
	DeleteClient(clientID string) (*gouaa.Client, error)
	ListClients(filter, by string, order gouaa.SortOrder, start, items int) ([]gouaa.Client, gouaa.Page, error)
}

func New(conf config.UAAConfig, trustedCert string, skipTLSValidation bool) (*Client, error) {
	var apiClient APIClient = &noopApiClient{}
	var err error

	if conf.Authentication.ClientCredentials.IsSet() {
		httpClient := newHTTPClient(trustedCert)
		apiClient, err = gouaa.New(
			conf.URL,
			gouaa.WithClientCredentials(
				conf.Authentication.ClientCredentials.ID,
				conf.Authentication.ClientCredentials.Secret,
				gouaa.JSONWebToken,
			),
			gouaa.WithClient(httpClient),
			gouaa.WithSkipSSLValidation(skipTLSValidation),
		)
	}

	return &Client{
		config:    conf,
		apiClient: apiClient,
		RandFunc:  randomString,
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

func (c *Client) HasClientDefinition() bool {
	cd := c.config.ClientDefinition
	return cd.Authorities != "" || cd.AuthorizedGrantTypes != "" || cd.Scopes != "" || cd.ResourceIDs != ""
}

func (c *Client) CreateClient(clientID, name, spaceGUID string) (map[string]string, error) {
	if !c.HasClientDefinition() {
		return nil, nil
	}

	grantTypes := c.config.ClientDefinition.AuthorizedGrantTypes
	allowPublic := strconv.FormatBool(c.config.ClientDefinition.AllowPublic)

	m := map[string]string{
		"client_id":              clientID,
		"scopes":                 interpolate(c.config.ClientDefinition.Scopes, map[string]string{odbSpaceGUID: spaceGUID}),
		"resource_ids":           c.config.ClientDefinition.ResourceIDs,
		"authorities":            c.config.ClientDefinition.Authorities,
		"authorized_grant_types": grantTypes,
		"allowpublic":            allowPublic,
	}

	if strings.Contains(grantTypes, "implicit") || strings.Contains(grantTypes, "authorization_code") {
		// UAA does not allow `implicit` o `authorization_code` clients to be created without a redirect uri
		m["redirect_uri"] = placeholderRedirectURI
	}

	var clientSecret string
	if !strings.Contains(grantTypes, "implicit") {
		if m["allowpublic"] == "true" {
			clientSecret = "-"
		} else {
			clientSecret = c.RandFunc()
		}
		m["client_secret"] = clientSecret
	}

	if c.config.ClientDefinition.Name != "" {
		m["name"] = c.config.ClientDefinition.Name
	} else {
		if name != "" {
			m["name"] = name
		}
	}

	resp, err := c.apiClient.CreateClient(c.transformToClient(m))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create uaa client")
	}

	return c.transformToMap(resp, clientSecret), nil
}

func (c *Client) UpdateClient(clientID string, redirectURI, spaceGUID string) (map[string]string, error) {
	if !c.HasClientDefinition() {
		return nil, nil
	}

	allowPublic := strconv.FormatBool(c.config.ClientDefinition.AllowPublic)
	interpolationMap := map[string]string{odbSpaceGUID: spaceGUID}
	m := map[string]string{
		"client_id":              clientID,
		"scopes":                 interpolate(c.config.ClientDefinition.Scopes, interpolationMap),
		"resource_ids":           interpolate(c.config.ClientDefinition.ResourceIDs, interpolationMap),
		"authorities":            interpolate(c.config.ClientDefinition.Authorities, interpolationMap),
		"authorized_grant_types": c.config.ClientDefinition.AuthorizedGrantTypes,
		"allowpublic":            allowPublic,
	}

	if c.config.ClientDefinition.Name != "" {
		m["name"] = c.config.ClientDefinition.Name
	}

	if redirectURI != "" {
		m["redirect_uri"] = redirectURI
	}

	resp, err := c.apiClient.UpdateClient(c.transformToClient(m))
	if err != nil {
		return nil, errors.Wrap(err, "failed to update uaa client")
	}
	return c.transformToMap(resp, ""), nil
}

func (c *Client) DeleteClient(clientID string) error {
	_, err := c.apiClient.DeleteClient(clientID)
	if err != nil {
		return errors.Wrap(err, "failed to delete client")
	}
	return err
}

func (c *Client) GetClient(clientID string) (map[string]string, error) {
	filter := fmt.Sprintf("client_id eq %q", clientID)
	existingClients, _, err := c.apiClient.ListClients(filter, "", "", 1, 1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list uaa clients")
	}
	if len(existingClients) == 0 {
		return nil, nil
	}

	return c.transformToMap(&existingClients[0], ""), nil
}

func (c *Client) transformToMap(resp *gouaa.Client, secret string) map[string]string {
	if resp == nil {
		return nil
	}
	return map[string]string{
		"client_id":              resp.ClientID,
		"client_secret":          secret, // client secret is not part of the response
		"name":                   resp.DisplayName,
		"scopes":                 fromSlice(resp.Scope),
		"resource_ids":           fromSlice(resp.ResourceIDs),
		"authorities":            fromSlice(resp.Authorities),
		"authorized_grant_types": fromSlice(resp.AuthorizedGrantTypes),
		"redirect_uri":           fromSlice(resp.RedirectURI),
		"allowpublic":            strconv.FormatBool(resp.AllowPublic),
	}
}

func (c *Client) transformToClient(m map[string]string) gouaa.Client {
	ap, _ := strconv.ParseBool(m["allowpublic"])
	client := gouaa.Client{
		Authorities:          toSlice(m["authorities"]),
		AuthorizedGrantTypes: toSlice(m["authorized_grant_types"]),
		ClientID:             m["client_id"],
		ClientSecret:         m["client_secret"],
		DisplayName:          m["name"],
		ResourceIDs:          toSlice(m["resource_ids"]),
		Scope:                toSlice(m["scopes"]),
		RedirectURI:          toSlice(m["redirect_uri"]),
		AllowPublic:          ap,
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

func interpolate(scopes string, m map[string]string) string {
	if scopes == "" {
		return ""
	}

	r := scopes
	for k, v := range m {
		r = strings.ReplaceAll(r, k, v)
	}
	return r
}

type noopApiClient struct{}

func (n *noopApiClient) CreateClient(_ gouaa.Client) (*gouaa.Client, error) {
	return nil, nil
}

func (n *noopApiClient) UpdateClient(_ gouaa.Client) (*gouaa.Client, error) {
	return nil, nil
}

func (n *noopApiClient) DeleteClient(_ string) (*gouaa.Client, error) {
	return nil, nil
}

func (n *noopApiClient) ListClients(_, _ string, _ gouaa.SortOrder, _, _ int) ([]gouaa.Client, gouaa.Page, error) {
	return nil, gouaa.Page{}, nil
}
