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

package on_demand_service_broker_test

import (
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("Shutdown of the broker process", func() {
	var (
		conf            brokerConfig.Config
		beforeTimingOut = time.Second / 10
		shutDownTimeout = 1
		shutDownChan    chan os.Signal
	)

	BeforeEach(func() {
		conf = brokerConfig.Config{
			Broker: brokerConfig.Broker{
				ShutdownTimeoutSecs: shutDownTimeout,
				Port:                serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				ID:   serviceID,
				Name: serviceName,
				Plans: brokerConfig.Plans{
					{Name: dedicatedPlanDisplayName, ID: dedicatedPlanID},
				},
			},
		}

		shouldSendSigterm = false
		shutDownChan = make(chan os.Signal, 1)
		StartServerWithStopHandler(conf, shutDownChan)
		setupFakeGenerateManifestOutput()
	})

	It("handles SIGTERM and exists gracefully", func() {
		killServer(shutDownChan)

		Eventually(loggerBuffer).Should(gbytes.Say("Broker shutting down on signal..."))
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	})

	It("waits for in-progress requests before exiting", func() {
		deployStarted := make(chan bool)
		deployFinished := make(chan bool)

		fakeTaskBoshClient.DeployStub = func(manifest []byte, contextID string, logger *log.Logger, reporter *boshdirector.AsyncTaskReporter) (int, error) {
			deployStarted <- true
			<-deployFinished
			return 0, nil
		}

		go func() {
			defer GinkgoRecover()
			resp, _ := doProvisionRequest("some-instance-id", dedicatedPlanID, nil, nil, true)
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		}()

		By("ensuring the operation is still in progress")
		Eventually(deployStarted).Should(Receive())

		By("send the SIGTERM signal")
		killServer(shutDownChan)

		By("ensuring the server received the signal")
		Eventually(loggerBuffer).Should(gbytes.Say("Broker shutting down on signal..."))

		By("ensuring the server is still running")
		Consistently(loggerBuffer, beforeTimingOut, 10*time.Millisecond).Should(SatisfyAll(
			Not(gbytes.Say("Server gracefully shut down")),
			Not(gbytes.Say("Error gracefully shutting down server")),
		))

		By("completing the operation")
		Expect(deployFinished).NotTo(Receive())
		deployFinished <- true

		By("gracefully terminating")
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	})

	It("kills the in-progress requests after the timeout period", func() {
		deployStarted := make(chan bool)
		deployFinished := make(chan bool)

		fakeTaskBoshClient.DeployStub = func(manifest []byte, contextID string, logger *log.Logger, reporter *boshdirector.AsyncTaskReporter) (int, error) {
			deployStarted <- true
			<-deployFinished
			return 0, errors.New("interrupted")
		}

		go func() {
			resp, _ := doProvisionRequest("some-instance-id", dedicatedPlanID, nil, nil, true)
			defer GinkgoRecover()
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		}()

		By("ensuring the operation is still in progress")
		Eventually(deployStarted).Should(Receive())

		By("send the SIGTERM signal")
		killServer(shutDownChan)

		By("ensuring the server received the signal")
		Eventually(loggerBuffer).Should(gbytes.Say("Broker shutting down on signal..."))

		By("timing out")
		Eventually(loggerBuffer, time.Second+time.Millisecond*100).Should(gbytes.Say("Error gracefully shutting down server"))

		deployFinished <- true
	})
})

func killServer(stopServer chan os.Signal) {
	stopServer <- syscall.SIGTERM
}
