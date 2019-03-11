// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"time"

	"github.com/craigfurman/herottp"
)

const (
	NotReadyYetExitCode = 10
	emptyListOfMetrics  = "[]"
)

func main() {
	brokerUsername := flag.String("brokerUsername", "", "username for the broker")
	brokerPassword := flag.String("brokerPassword", "", "password for the broker")
	brokerUrl := flag.String("brokerUrl", "", "url of the broker")
	tlsCertificate := flag.String("tlsCertificate", "", "broker certificate")
	disableSSLCertVerification := flag.Bool("disableSSLCertVerification", false, "set to true to disable SSL validation on communication with the broker")
	flag.Parse()

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		fatalError(err)
	}
	rootCAs.AppendCertsFromPEM([]byte(*tlsCertificate))

	brokerMetricsUrl := *brokerUrl + "/mgmt/metrics"
	client := herottp.New(herottp.Config{
		Timeout:                           30 * time.Second,
		RootCAs:                           rootCAs,
		DisableTLSCertificateVerification: *disableSSLCertVerification,
	})

	request, err := http.NewRequest("GET", brokerMetricsUrl, nil)
	if err != nil {
		fatalError(err)
	}
	request.SetBasicAuth(*brokerUsername, *brokerPassword)
	response, err := client.Do(request)
	if err != nil {
		fatalError(err)
	}

	switch response.StatusCode {
	case http.StatusInternalServerError:
		fmt.Print(emptyListOfMetrics)
		os.Exit(0)
	case http.StatusServiceUnavailable:
		os.Exit(NotReadyYetExitCode)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fatalError(err)
	}

	fmt.Print(string(body))
}

func fatalError(err error) {
	fmt.Printf("error collecting metrics: %s", err)
	os.Exit(1)
}
