// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credhub_tests

import (
	"fmt"
	"os"

	"log"
	"testing"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
	odbcredhub "github.com/pivotal-cf/on-demand-service-broker/credhub"

	"crypto/x509"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/totherme/unstructured"
)

const credhubURL = "https://credhub.service.cf.internal:8844"

var (
	dev_env string
	caCerts []string
)

func TestContractTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhub Contract Tests Suite")
}

var _ = BeforeSuite(func() {
	dev_env = os.Getenv("DEV_ENV")
	caCerts = extractCAsFromManifest()
	ensureCredhubIsClean()
})

var _ = AfterSuite(func() {
	ensureCredhubIsClean()
})

func getBoshManifest(deploymentName string) ([]byte, error) {
	logger := log.New(GinkgoWriter, "", loggerfactory.Flags)
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return []byte{}, err
	}

	boshURL := os.Getenv("BOSH_URL")
	Expect(boshURL).NotTo(BeEmpty(), "Expected BOSH_URL to be set")

	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := boshdir.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)

	username := os.Getenv("BOSH_USERNAME")
	Expect(username).NotTo(BeEmpty(), "Expected BOSH_USERNAME to be set")
	password := os.Getenv("BOSH_PASSWORD")
	Expect(password).NotTo(BeEmpty(), "Expected BOSH_PASSWORD to be set")
	boshCert := os.Getenv("BOSH_CA_CERT")
	Expect(boshCert).NotTo(BeEmpty(), "Expected BOSH_CA_CERT to be set")

	boshClient, err := boshdirector.New(
		boshURL,
		[]byte(boshCert),
		certPool,
		directorFactory,
		uaaFactory,
		config.Authentication{UAA: config.UAAAuthentication{ClientCredentials: config.ClientCredentials{ID: username, Secret: password}}},
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
		logger,
	)

	if err != nil {
		return []byte{}, err
	}

	cfRawManifest, exists, err := boshClient.GetDeployment(deploymentName, logger)
	if err != nil {
		return []byte{}, err
	}

	if !exists {
		return []byte{}, fmt.Errorf("deployment '%s' not found", deploymentName)
	}
	return cfRawManifest, nil
}

func nameIsCredhubMatcher(data unstructured.Data) bool {
	val, err := data.GetByPointer("/name")
	if err != nil {
		return false
	}
	stringVal, err := val.StringValue()
	if err != nil {
		return false
	}
	return stringVal == "credhub"
}

func extractCAsFromManifest() []string {
	cfRawManifest, err := getBoshManifest("cf")
	Expect(err).NotTo(HaveOccurred())

	cfManifest, err := unstructured.ParseYAML(string(cfRawManifest))
	Expect(err).NotTo(HaveOccurred())

	igs, err := cfManifest.GetByPointer("/instance_groups")
	Expect(err).NotTo(HaveOccurred())

	credhubGroup, found := igs.FindElem(nameIsCredhubMatcher)
	Expect(found).To(BeTrue())

	jobs, err := credhubGroup.GetByPointer("/jobs")
	Expect(err).NotTo(HaveOccurred())
	credhubJob, found := jobs.FindElem(nameIsCredhubMatcher)
	Expect(found).To(BeTrue())

	credhubProperties, err := credhubJob.GetByPointer("/properties/credhub")
	Expect(err).NotTo(HaveOccurred())

	uaaCert, err := credhubProperties.GetByPointer("/authentication/uaa/ca_certs/0")
	Expect(err).NotTo(HaveOccurred())
	credhubCert, err := credhubProperties.GetByPointer("/tls/ca")
	Expect(err).NotTo(HaveOccurred())

	return []string{uaaCert.UnsafeStringValue(), credhubCert.UnsafeStringValue()}
}

func testKeyPrefix() string {
	return fmt.Sprintf("/test-%s", dev_env)
}

func makeKeyPath(name string) string {
	return fmt.Sprintf("%s/%s", testKeyPrefix(), name)
}

func ensureCredhubIsClean() {
	credhubClient := underlyingCredhubClient()

	testKeys, err := credhubClient.FindByPath(testKeyPrefix())
	Expect(err).NotTo(HaveOccurred())
	for _, key := range testKeys.Credentials {
		credhubClient.Delete(key.Name)
	}
}

func getCredhubStore() *odbcredhub.Store {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	Expect(clientSecret).NotTo(BeEmpty(), "Expected TEST_CREDHUB_CLIENT_SECRET to be set")

	credentialStore, err := odbcredhub.Build(
		credhubURL,
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
		credhub.CaCerts(caCerts...),
	)
	Expect(err).NotTo(HaveOccurred())
	return credentialStore
}

func underlyingCredhubClient() *credhub.CredHub {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	Expect(clientSecret).NotTo(BeEmpty(), "Expected TEST_CREDHUB_CLIENT_SECRET to be set")
	Expect(caCerts).ToNot(BeEmpty())

	credhubClient, err := credhub.New(
		credhubURL,
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
		credhub.CaCerts(caCerts...),
	)
	Expect(err).NotTo(HaveOccurred())
	return credhubClient
}
