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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type BoshHelperClient struct {
	*boshdirector.Client
}

func New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert string) *BoshHelperClient {
	var boshCACertContents []byte
	if boshCACert != "" {
		var err error
		boshCACertContents, err = ioutil.ReadFile(boshCACert)
		Expect(err).NotTo(HaveOccurred())
	}

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())
	certPool.AppendCertsFromPEM(boshCACertContents)

	logger := systemTestLogger()
	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)

	boshClient, err := boshdirector.New(
		boshURL,
		boshCACertContents,
		certPool,
		directorFactory,
		uaaFactory,
		config.Authentication{
			UAA: config.UAAAuthentication{
				URL: uaaURL,
				ClientCredentials: config.ClientCredentials{
					ID: boshUsername, Secret: boshPassword,
				},
			},
		},
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
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

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	logger := systemTestLogger()
	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)
	boshClient, err := boshdirector.New(
		boshURL,
		boshCACertContents,
		certPool,
		directorFactory,
		uaaFactory,
		config.Authentication{
			Basic: config.UserCredentials{
				Username: boshUsername, Password: boshPassword,
			},
		},
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
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
	taskReporter := boshdirector.NewAsyncTaskReporter()
	taskID, err := b.Client.RunErrand(deploymentName, errandName, errandInstances, contextID, logger, taskReporter)
	Expect(err).NotTo(HaveOccurred())
	<-taskReporter.Finished
	return taskID
}

func (b *BoshHelperClient) getTaskOutput(taskID int, logger *log.Logger) boshdirector.BoshTaskOutput {
	output, err := b.Client.GetTaskOutput(taskID, logger)
	Expect(err).NotTo(HaveOccurred())
	return output
}

func (b *BoshHelperClient) GetTasksForDeployment(deploymentName string) boshdirector.BoshTasks {
	logger := systemTestLogger()
	boshTasks, err := b.Client.GetTasks(deploymentName, logger)
	Expect(err).NotTo(HaveOccurred())
	return boshTasks
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

	asyncReporter := boshdirector.NewAsyncTaskReporter()
	taskID, err := b.Client.Deploy(manifestBytes, "", logger, asyncReporter)
	Expect(err).NotTo(HaveOccurred())

	<-asyncReporter.Finished

	task, err := b.Client.GetTask(taskID, logger)
	Expect(err).NotTo(HaveOccurred())
	Expect(task.StateType()).To(Equal(boshdirector.TaskComplete))

	// wait for Broker Route Registration Interval
	time.Sleep(30 * time.Second)
}

func (b *BoshHelperClient) DeploymentExists(deploymentName string) bool {
	logger := systemTestLogger()
	_, found, err := b.Client.GetDeployment(deploymentName, logger)
	Expect(err).NotTo(HaveOccurred())
	return found
}

func (b *BoshHelperClient) DeleteDeployment(deploymentName string) {
	logger := systemTestLogger()
	taskReporter := boshdirector.NewAsyncTaskReporter()
	_, err := b.Client.DeleteDeployment(deploymentName, "", false, taskReporter, logger)
	Expect(err).NotTo(HaveOccurred())
	<-taskReporter.Finished
}

func systemTestLogger() *log.Logger {
	return log.New(GinkgoWriter, "[system tests boshdirector] ", log.LstdFlags)
}

func FindJobProperties(brokerManifest *bosh.BoshManifest, igName, jobName string) map[string]interface{} {
	job := FindJob(brokerManifest, igName, jobName)
	return job.Properties
}

func FindJob(brokerManifest *bosh.BoshManifest, igName, jobName string) bosh.Job {
	for _, job := range FindInstanceGroupJobs(brokerManifest, igName) {
		if job.Name == jobName {
			return job
		}
	}
	return bosh.Job{}
}

func FindInstanceGroupProperties(manifest *bosh.BoshManifest, igName string) map[string]interface{} {
	ig := FindInstanceGroup(manifest, igName)
	return ig.Properties
}

func FindInstanceGroupJobs(manifest *bosh.BoshManifest, igName string) []bosh.Job {
	ig := FindInstanceGroup(manifest, igName)
	return ig.Jobs
}

func FindInstanceGroup(manifest *bosh.BoshManifest, igName string) bosh.InstanceGroup {
	for _, ig := range manifest.InstanceGroups {
		if ig.Name == igName {
			return ig
		}
	}
	return bosh.InstanceGroup{}
}
