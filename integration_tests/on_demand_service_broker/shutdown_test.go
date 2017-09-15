// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"syscall"
	"time"

	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var _ = Describe("broker process", func() {

	const instanceID = "some-instance-id"

	var (
		runningBroker *gexec.Session
		boshDirector  *mockhttp.Server
		cfAPI         *mockhttp.Server
		boshUAA       *mockuaa.ClientCredentialsServer
		cfUAA         *mockuaa.ClientCredentialsServer
		brokerConfig  config.Config
		manifest      bosh.BoshManifest
	)

	BeforeEach(func() {
		assertNoRunningBroker()

		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.New()
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		brokerConfig = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		adapter.DashboardUrlGenerator().NotImplemented()

		manifest = bosh.BoshManifest{
			Name:           deploymentName(instanceID),
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
		adapter.GenerateManifest().ToReturnManifest(rawManifestFromBoshManifest(manifest))
	})

	AfterEach(func() {
		cfAPI.VerifyMocks()
		boshDirector.VerifyMocks()

		runningBroker.Kill()
		assertNoRunningBroker()
	})

	It("handles SIGTERM and exits gracefully", func() {
		runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		runningBroker.Signal(syscall.SIGTERM)
		Eventually(runningBroker.Out).Should(gbytes.Say("Broker shutting down on signal..."))
		Eventually(runningBroker.Out).Should(gbytes.Say("Server gracefully shut down"))
		Eventually(runningBroker).Should(gexec.Exit(0))
	})

	It("waits for in-progress requests before exiting", func() {
		runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)

		deployStarted := make(chan bool, 1)
		deployFinished := make(chan bool, 1)
		pauseDeploy := make(chan bool)

		boshDirector.VerifyAndMock(
			respondsWithDeploymentNotFound(instanceID),
			respondsWithNoTasks(instanceID),
			mockbosh.
				DeploysWithManifestAndRedirectsToTask(manifest, 666).
				SendToChannel(deployStarted).
				WaitForChannel(pauseDeploy).
				SendToChannel(deployFinished),
		)

		go func() {
			defer GinkgoRecover()
			provisionInstance(instanceID, highMemoryPlanID, map[string]interface{}{})
		}()

		// make sure we are inside the bosh deploy request
		Eventually(deployStarted).Should(Receive())

		runningBroker.Signal(syscall.SIGTERM)

		// broker should not exit because deploy is in progress
		shutdownTimeout := time.Second * time.Duration(brokerConfig.Broker.ShutdownTimeoutSecs)
		justBeforeTimeout := shutdownTimeout - time.Millisecond*50
		Consistently(runningBroker, justBeforeTimeout, 10*time.Millisecond).ShouldNot(gexec.Exit())

		// deploy should still be waiting
		Expect(deployFinished).NotTo(Receive())

		close(pauseDeploy)

		Eventually(deployFinished).Should(Receive())
		Eventually(runningBroker).Should(gexec.Exit(0))
	})

	It("when broker is processing a long running provision request sending SIGTERM should cancel the request after the timeout period", func() {
		runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)

		taskID := 4
		deployTriggered := make(chan bool)
		shutdownTimeout := time.Second * time.Duration(brokerConfig.Broker.ShutdownTimeoutSecs)
		delay := shutdownTimeout + time.Second

		mockBoshDeployWithTriggerAndDelay(
			boshDirector,
			instanceID,
			manifest,
			taskID,
			deployTriggered,
			delay,
		)

		var wg sync.WaitGroup
		go expectProvisionToFail(&wg, instanceID)

		terminateWhenTriggered(runningBroker, deployTriggered)

		wg.Wait()
	})
})

func mockBoshDeployWithTriggerAndDelay(
	director *mockhttp.Server,
	instanceID string,
	manifest bosh.BoshManifest,
	taskID int,
	trigger chan bool,
	delay time.Duration,
) {

	director.VerifyAndMock(
		respondsWithNoDeployment(instanceID),
		respondsWithNoTasks(instanceID),
		mockbosh.DeploysWithManifestAndRedirectsToTask(manifest, taskID).
			SendToChannel(trigger).
			DelayResponse(delay),
	)
}

func assertNoRunningBroker() {
	Eventually(dialBroker).Should(BeFalse(), "an old instance of the broker is still running")
}

func terminateWhenTriggered(process *gexec.Session, trigger chan bool) {
	<-trigger
	process.Terminate()
}

func expectProvisionToFail(w *sync.WaitGroup, instanceID string) {
	defer GinkgoRecover()
	defer w.Done()

	w.Add(1)

	_, err := provisionInstanceWithAsyncFlag(
		instanceID,
		highMemoryPlanID,
		map[string]interface{}{},
		true,
	)

	Expect(err).To(HaveOccurred())
}

func respondsWithNoDeployment(instanceID string) *mockhttp.Handler {
	return mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith("")
}

func respondsWithDeploymentNotFound(instanceID string) *mockhttp.Handler {
	return mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith("")
}

func respondsWithNoTasks(instanceID string) *mockhttp.Handler {
	return mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks()
}
