package siapi_helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/onsi/ginkgo"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type SIAPIConfig struct {
	URL      string
	Username string
	Password string
}

// TODO: use this in Upgrade test (remove duplicate method)

func UpdateServiceInstancesAPI(serviceApiConfig SIAPIConfig, instances []service.Instance) error {
	instancesJson, err := json.Marshal(instances)
	if err != nil {
		return err
	}

	url := strings.Replace(serviceApiConfig.URL, "https", "http", 1)

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
	})

	basicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		serviceApiConfig.Username,
		serviceApiConfig.Password,
	)

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(instancesJson))
	if err != nil {
		return err
	}

	logger := log.New(ginkgo.GinkgoWriter, "", log.LstdFlags)
	if err = basicAuthHeaderBuilder.AddAuthHeader(request, logger); err != nil {
		return err
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code %d from SI API", resp.StatusCode)
	}
	return nil
}
