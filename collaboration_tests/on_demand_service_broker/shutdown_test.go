package on_demand_service_broker_test

import (
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
		shutDownChan    chan os.Signal
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
		shutDownChan = make(chan os.Signal, 1)
		StartServerWithStopHandler(conf, shutDownChan)
	})

	It("handles SIGTERM and exists gracefully", func() {
		killServer(shutDownChan)

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
			resp, _ := doProvisionRequest("some-instance-id", "some-plan", nil, true)
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

		fakeDeployer.CreateStub = func(name, id string, reqParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error) {
			deployStarted <- true
			<-deployFinished
			return 0, nil, errors.New("interrupted")
		}

		go func() {
			resp, _ := doProvisionRequest("some-instance-id", "some-plan", nil, true)
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
