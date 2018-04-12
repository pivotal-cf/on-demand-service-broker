package on_demand_service_broker_test

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	"os"
	"syscall"
	"testing"

	"math/rand"

	"math"

	"io"
	"io/ioutil"
	"net/http"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
	credhubfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func TestOnDemandServiceBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OnDemandServiceBroker Collaboration Test Suite")
}

const (
	componentName  = "collaboration-tests"
	serviceName    = "service-name"
	brokerUsername = "username"
	brokerPassword = "password"
)

var (
	stopServer                 chan os.Signal
	serverPort                 = rand.Intn(math.MaxInt16-1024) + 1024
	serverURL                  = fmt.Sprintf("localhost:%d", serverPort)
	fakeServiceAdapter         *fakes.FakeServiceAdapterClient
	fakeCredentialStoreFactory *credhubfakes.FakeCredentialStoreFactory
	fakeCredentialStore        *credhubfakes.FakeCredentialStore
	fakeBoshClient             *fakes.FakeBoshClient
	fakeCfClient               *fakes.FakeCloudFoundryClient
	fakeDeployer               *fakes.FakeDeployer
	loggerBuffer               *gbytes.Buffer
	shouldSendSigterm          bool
)

var _ = BeforeEach(func() {
	fakeBoshClient = new(fakes.FakeBoshClient)
	fakeServiceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeCredentialStoreFactory = new(credhubfakes.FakeCredentialStoreFactory)
	fakeCredentialStore = new(credhubfakes.FakeCredentialStore)
	fakeCfClient = new(fakes.FakeCloudFoundryClient)
	fakeDeployer = new(fakes.FakeDeployer)

	fakeCredentialStoreFactory.NewReturns(fakeCredentialStore, nil)
})

var _ = AfterEach(func() {
	if shouldSendSigterm {
		stopServer <- syscall.SIGTERM
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	}
})

func StartServer(conf config.Config) {
	stopServer = make(chan os.Signal, 1)
	conf.Broker.ShutdownTimeoutSecs = 1
	shouldSendSigterm = true
	StartServerWithStopHandler(conf, stopServer)
}

func StartServerWithStopHandler(conf config.Config, stopServerChan chan os.Signal) {
	loggerBuffer = gbytes.NewBuffer()
	loggerFactory := loggerfactory.New(loggerBuffer, componentName, loggerfactory.Flags)
	logger := loggerFactory.New()
	fakeOnDemandBroker, err := broker.New(
		fakeBoshClient,
		fakeCfClient,
		conf.ServiceCatalog,
		conf.Broker,
		nil,
		fakeServiceAdapter,
		fakeDeployer,
		loggerFactory,
	)
	Expect(err).NotTo(HaveOccurred())
	fakeBroker, err := brokeraugmenter.New(conf, fakeOnDemandBroker, fakeCredentialStoreFactory, loggerFactory)
	Expect(err).NotTo(HaveOccurred())
	server := apiserver.New(
		conf,
		fakeBroker,
		componentName,
		loggerFactory,
		logger,
	)
	go apiserver.StartAndWait(conf, server, logger, stopServerChan)
	Eventually(func() bool {
		conn, _ := net.Dial("tcp", serverURL)
		if conn != nil {
			Expect(conn.Close()).To(Succeed())
			return true
		}
		return false
	}).Should(BeTrue(), "Server did not start")
	Expect(loggerBuffer).To(gbytes.Say("Listening on"))
}

func doRequest(method, url string, body io.Reader, requestModifiers ...func(r *http.Request)) (*http.Response, []byte) {
	req, err := http.NewRequest(method, url, body)
	Expect(err).ToNot(HaveOccurred())

	req.SetBasicAuth(brokerUsername, brokerPassword)
	req.Header.Set("X-Broker-API-Version", "2.14")

	for _, f := range requestModifiers {
		f(req)
	}

	req.Close = true
	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	Expect(resp.Body.Close()).To(Succeed())
	return resp, bodyContent
}
