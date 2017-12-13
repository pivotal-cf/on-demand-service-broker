package on_demand_service_broker_test

import (
	"fmt"
	"log"
	"os"

	"syscall"

	"net/http"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"
)

var _ = Describe("Shutdown of the broker process", func() {
	var (
		conf            brokerConfig.Config
		beforeTimingOut = time.Second / 10
		shutDownTimeout = 1
	)

	BeforeEach(func() {

		conf = brokerConfig.Config{
			Broker: brokerConfig.Broker{
				ShutdownTimeoutSecs: shutDownTimeout,
				Port:                serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				Plans: brokerConfig.Plans{
					{Name: "some-plan", ID: "some-plan"},
				},
			},
		}

		shouldSendSigterm = false
		stopServer = make(chan os.Signal, 1)
		StartServerWithStopHandler(conf, stopServer)
		Eventually(loggerBuffer).Should(gbytes.Say("Listening on"))
	})

	It("handles SIGTERM and exists gracefully", func() {
		killServer(serverURL, stopServer)

		Eventually(loggerBuffer).Should(gbytes.Say("Broker shutting down on signal..."))
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	})

	It("waits for in-progress requests before exiting", func() {
		deployStarted := make(chan bool)
		deployFinished := make(chan bool)

		fakeDeployer.CreateStub = func(name, id string, reqParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error) {
			deployStarted <- true
			<-deployFinished
			return 0, nil, nil
		}

		go func() {
			resp := doProvisionRequest("some-instance-id", "some-plan", nil, true)
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		}()

		By("ensuring the operation is still in progress")
		Eventually(deployStarted).Should(Receive())

		By("send the SIGTERM signal")
		killServer(serverURL, stopServer)

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

		fakeDeployer.CreateStub = func(name, id string, reqParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error) {
			deployStarted <- true
			<-deployFinished
			return 0, nil, errors.New("interrupted")
		}

		go func() {
			resp := doProvisionRequest("some-instance-id", "some-plan", nil, true)
			defer GinkgoRecover()
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		}()

		By("ensuring the operation is still in progress")
		Eventually(deployStarted).Should(Receive())

		By("send the SIGTERM signal")
		killServer(serverURL, stopServer)

		By("ensuring the server received the signal")
		Eventually(loggerBuffer).Should(gbytes.Say("Broker shutting down on signal..."))

		By("timing out")
		Eventually(loggerBuffer, time.Second+time.Millisecond*100).Should(gbytes.Say("Error gracefully shutting down server"))

		deployFinished <- true
	})
})

func killServer(serverURL string, stopServer chan os.Signal) {
	stopServer <- syscall.SIGTERM
}

func isRunning(serverURL string) bool {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/mgmt/service_instances", serverURL), nil)
	Expect(err).ToNot(HaveOccurred())
	req.SetBasicAuth(brokerUsername, brokerPassword)

	_, err = http.DefaultClient.Do(req)
	return err == nil
}
