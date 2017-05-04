// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package bosh_helpers

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type BoshHelperClient struct {
	*boshclient.Client
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
	boshClient, err := boshclient.New(boshURL, authHeaderBuilder, false, boshCACertContents)
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
	boshClient, err := boshclient.New(boshURL, basicAuthHeaderBuilder, disableTLSVerification, boshCACertContents)
	Expect(err).NotTo(HaveOccurred())
	return &BoshHelperClient{Client: boshClient}
}

func (b *BoshHelperClient) RunErrand(deploymentName, errandName, contextID string) boshclient.BoshTaskOutput {
	logger := systemTestLogger()
	taskID := b.runErrandAndWait(deploymentName, errandName, contextID, logger)
	output := b.getTaskOutput(taskID, logger)
	Expect(output.ExitCode).To(BeZero(), fmt.Sprintf("STDOUT: ------------\n%s\n--------------------\nSTDERR: ------------\n%s\n--------------------\n", output.StdOut, output.StdErr))
	return output
}

func (b *BoshHelperClient) RunErrandWithoutCheckingSuccess(deploymentName, errandName, contextID string) boshclient.BoshTaskOutput {
	logger := systemTestLogger()
	taskID := b.runErrandAndWait(deploymentName, errandName, contextID, logger)
	return b.getTaskOutput(taskID, logger)
}

func (b *BoshHelperClient) runErrandAndWait(deploymentName, errandName, contextID string, logger *log.Logger) int {
	taskID, err := b.Client.RunErrand(deploymentName, errandName, contextID, logger)
	Expect(err).NotTo(HaveOccurred())
	b.waitForTaskToFinish(taskID)
	return taskID
}

func (b *BoshHelperClient) getTaskOutput(taskID int, logger *log.Logger) boshclient.BoshTaskOutput {
	output, err := b.Client.GetTaskOutput(taskID, logger)
	Expect(err).NotTo(HaveOccurred())
	Expect(output).To(HaveLen(1))
	return output[0]
}

func (b *BoshHelperClient) GetTasksForDeployment(deploymentName string) boshclient.BoshTasks {
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

		if taskState.State == boshclient.BoshTaskError {
			Fail(fmt.Sprintf("task %d failed: %s", taskID, taskState.Description))
		}

		if taskState.State == boshclient.BoshTaskDone {
			break
		}

		time.Sleep(time.Second * b.BoshPollingInterval)
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
	return log.New(GinkgoWriter, "[system tests boshclient] ", log.LstdFlags)
}
