// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package bosh_helpers

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/craigfurman/herottp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type BoshHelperClient struct {
	*boshdirector.Client
}

type authenticatorBuilder struct {
	authHeaderBuilder boshdirector.AuthHeaderBuilder
}

func (a authenticatorBuilder) NewAuthHeaderBuilder(uaaURL string, disableSSL bool) (config.AuthHeaderBuilder, error) {
	return a.authHeaderBuilder, nil
}

func New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert string) *BoshHelperClient {
	var boshCACertContents []byte
	if boshCACert != "" {
		var err error
		boshCACertContents, err = ioutil.ReadFile(boshCACert)
		Expect(err).NotTo(HaveOccurred())
	}

	authHeaderBuilder, err := authorizationheader.NewClientTokenAuthHeaderBuilder(uaaURL, boshUsername, boshPassword, false, boshCACertContents)
	Expect(err).NotTo(HaveOccurred())

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())
	certPool.AppendCertsFromPEM(boshCACertContents)

	httpClient := herottp.New(herottp.Config{
		NoFollowRedirect:                  true,
		DisableTLSCertificateVerification: false,
		RootCAs: certPool,
		Timeout: 30 * time.Second,
	})

	logger := systemTestLogger()
	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)

	boshClient, err := boshdirector.New(
		boshURL,
		false,
		boshCACertContents,
		httpClient,
		authenticatorBuilder{authHeaderBuilder},
		certPool,
		directorFactory,
		uaaFactory,
		config.BOSHAuthentication{
			UAA: config.BOSHUAAAuthentication{
				ID: boshUsername, Secret: boshPassword,
			},
		},
		logger,
	)

	Expect(err).NotTo(HaveOccurred())
	return &BoshHelperClient{Client: boshClient}
}

func NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert string, disableTLSVerification bool) *BoshHelperClient {
	var boshCACertContents []byte
	if boshCACert != "" {
		var err error
		boshCACertContents, err = ioutil.ReadFile(boshCACert)
		Expect(err).NotTo(HaveOccurred())
	}

	basicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(boshUsername, boshPassword)
	var err error
	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	httpClient := herottp.New(herottp.Config{
		NoFollowRedirect:                  true,
		DisableTLSCertificateVerification: false,
		RootCAs: certPool,
		Timeout: 30 * time.Second,
	})

	logger := systemTestLogger()
	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)
	boshClient, err := boshdirector.New(
		boshURL,
		disableTLSVerification,
		boshCACertContents,
		httpClient,
		authenticatorBuilder{basicAuthHeaderBuilder},
		certPool,
		directorFactory,
		uaaFactory,
		config.BOSHAuthentication{
			Basic: config.UserCredentials{
				Username: boshUsername, Password: boshPassword,
			},
		},
		logger,
	)

	Expect(err).NotTo(HaveOccurred())

	return &BoshHelperClient{Client: boshClient}
}

func (b *BoshHelperClient) RunErrand(deploymentName string, errandName string, errandInstances []string, contextID string) boshdirector.BoshTaskOutput {
	output := b.RunErrandWithoutCheckingSuccess(deploymentName, errandName, errandInstances, contextID)
	Expect(output.ExitCode).To(BeZero(), fmt.Sprintf("STDOUT: ------------\n%s\n--------------------\nSTDERR: ------------\n%s\n--------------------\n", output.StdOut, output.StdErr))
	return output
}

func (b *BoshHelperClient) RunErrandWithoutCheckingSuccess(deploymentName string, errandName string, errandInstances []string, contextID string) boshdirector.BoshTaskOutput {
	logger := systemTestLogger()
	taskID := b.runErrandAndWait(deploymentName, errandName, errandInstances, contextID, logger)
	return b.getTaskOutput(taskID, logger)
}

func (b *BoshHelperClient) runErrandAndWait(deploymentName string, errandName string, errandInstances []string, contextID string, logger *log.Logger) int {
	taskID, err := b.Client.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
	Expect(err).NotTo(HaveOccurred())
	b.waitForTaskToFinish(taskID)
	return taskID
}

func (b *BoshHelperClient) getTaskOutput(taskID int, logger *log.Logger) boshdirector.BoshTaskOutput {
	output, err := b.Client.GetTaskOutput(taskID, logger)
	Expect(err).NotTo(HaveOccurred())
	Expect(output).To(HaveLen(1))
	return output[0]
}

func (b *BoshHelperClient) GetTasksForDeployment(deploymentName string) boshdirector.BoshTasks {
	logger := systemTestLogger()
	boshTasks, err := b.Client.GetTasks(deploymentName, logger)
	Expect(err).NotTo(HaveOccurred())
	return boshTasks
}

func (b *BoshHelperClient) waitForTaskToFinish(taskID int) {
	logger := systemTestLogger()
	for {
		taskState, err := b.Client.GetTask(taskID, logger)
		Expect(err).NotTo(HaveOccurred())

		if taskState.State == boshdirector.TaskError {
			Fail(fmt.Sprintf("task %d failed: %s", taskID, taskState.Description))
		}

		if taskState.State == boshdirector.TaskDone {
			break
		}

		time.Sleep(time.Second * b.PollingInterval)
	}
}

func (b *BoshHelperClient) GetManifest(deploymentName string) *bosh.BoshManifest {
	logger := systemTestLogger()

	data, found, err := b.Client.GetDeployment(deploymentName, logger)
	Expect(err).NotTo(HaveOccurred())

	if !found {
		return nil
	}

	var manifest bosh.BoshManifest
	err = yaml.Unmarshal(data, &manifest)
	Expect(err).NotTo(HaveOccurred())

	return &manifest
}

func (b *BoshHelperClient) DeployODB(manifest bosh.BoshManifest) {
	logger := systemTestLogger()

	manifestBytes, err := yaml.Marshal(manifest)
	Expect(err).NotTo(HaveOccurred())

	deployTaskID, err := b.Client.Deploy(manifestBytes, "", logger)
	Expect(err).NotTo(HaveOccurred())

	b.waitForTaskToFinish(deployTaskID)

	// wait for Broker Route Registration Interval
	time.Sleep(20 * time.Second)
}

func (b *BoshHelperClient) DeploymentExists(deploymentName string) bool {
	logger := systemTestLogger()
	_, found, err := b.Client.GetDeployment(deploymentName, logger)
	Expect(err).NotTo(HaveOccurred())
	return found
}

func (b *BoshHelperClient) DeleteDeployment(deploymentName string) {
	logger := systemTestLogger()
	deleteTaskID, err := b.Client.DeleteDeployment(deploymentName, "", logger)
	Expect(err).NotTo(HaveOccurred())
	b.waitForTaskToFinish(deleteTaskID)
}

func systemTestLogger() *log.Logger {
	return log.New(GinkgoWriter, "[system tests boshdirector] ", log.LstdFlags)
}
