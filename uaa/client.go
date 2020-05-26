package uaa

import "github.com/pivotal-cf/on-demand-service-broker/config"

type NoopClient struct{}

func (n *NoopClient) CreateClient() (map[string]string, error) {
	return nil, nil
}

type Client struct{}

func New(config config.UAAConfig) *Client {
	return &Client{}
}

func (n *Client) CreateClient() (map[string]string, error) {
	return nil, nil
}
