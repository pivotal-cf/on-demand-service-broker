// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
)

const (
	OrphanBoshDeploymentsDetectedMessage = "Orphan BOSH deployments detected with no corresponding service instance in Cloud Foundry. Before deleting any deployment it is recommended to verify the service instance no longer exists in Cloud Foundry and any data is safe to delete."
	OrphanDeploymentsDetectedExitCode    = 10
)

func main() {
	brokerUsername := flag.String("brokerUsername", "", "username for the broker")
	brokerPassword := flag.String("brokerPassword", "", "password for the broker")
	brokerURL := flag.String("brokerUrl", "", "url of the broker")
	flag.Parse()

	orphanDeploymentsURL := fmt.Sprintf("%s/mgmt/orphan_deployments", *brokerURL)
	request, err := http.NewRequest("GET", orphanDeploymentsURL, nil)
	if err != nil {
		log.Fatalf("invalid broker URL: %s", *brokerURL)
	}
	request.SetBasicAuth(*brokerUsername, *brokerPassword)

	client := herottp.New(herottp.Config{Timeout: 30 * time.Second})

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("orphan deployments request error: %s", err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("error reading response body: %s. status code: %d.", err, response.StatusCode)
	}

	if response.StatusCode != http.StatusOK {
		log.Fatalf(
			"orphan deployments request error. status code: %d. body: '%s'.",
			response.StatusCode,
			body,
		)
	}

	var orphanDeployments []mgmtapi.Deployment
	err = json.Unmarshal(body, &orphanDeployments)
	if err != nil {
		log.Fatalf("error decoding JSON response: %s. status code: %d.", err, response.StatusCode)
	}

	fmt.Fprint(os.Stdout, string(body))

	if len(orphanDeployments) > 0 {
		fmt.Fprint(os.Stderr, OrphanBoshDeploymentsDetectedMessage)
		os.Exit(OrphanDeploymentsDetectedExitCode)
	}
}
