package uaa

import (
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"math/rand"
	"time"
)

type NoopClient struct{}

func (n *NoopClient) CreateClient(_, _ string) (map[string]string, error) {
	return nil, nil
}

type Client struct {
	config config.UAAConfig
}

func New(config config.UAAConfig) *Client {
	return &Client{config: config}
}

func (c *Client) CreateClient(id, name string) (map[string]string, error) {
	m := map[string]string{
		"client_id":              id,
		"client_secret":          randomString(),
		"scopes":                 c.config.ClientDefinition.Scopes,
		"resource_ids":           c.config.ClientDefinition.ResourceIDs,
		"authorities":            c.config.ClientDefinition.Authorities,
		"authorized_grant_types": c.config.ClientDefinition.AuthorizedGrantTypes,
	}

	if name != "" {
		m["name"] = name
	}

	return m, nil
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
