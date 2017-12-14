package on_demand_service_broker_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	"os"
	"syscall"
	"testing"

	"math/rand"

	"math"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
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
	stopServer         chan os.Signal
	serverPort         = rand.Intn(math.MaxInt16-1024) + 1024
	serverURL          = fmt.Sprintf("localhost:%d", serverPort)
	fakeServiceAdapter *fakes.FakeServiceAdapterClient
	fakeBoshClient     *fakes.FakeBoshClient
	fakeCfClient       *fakes.FakeCloudFoundryClient
	fakeDeployer       *fakes.FakeDeployer
	loggerBuffer       *gbytes.Buffer
	shouldSendSigterm  bool
)

var _ = BeforeEach(func() {
	fakeBoshClient = new(fakes.FakeBoshClient)
	fakeServiceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeCfClient = new(fakes.FakeCloudFoundryClient)
	fakeDeployer = new(fakes.FakeDeployer)

})

var _ = AfterEach(func() {
	if shouldSendSigterm {
		stopServer <- syscall.SIGTERM
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	}
})

func StartServer(conf config.Config) {
	stopServer = make(chan os.Signal, 1)
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
		nil,
		fakeServiceAdapter,
		fakeDeployer,
		loggerFactory,
	)
	Expect(err).NotTo(HaveOccurred())
	fakeBroker, err := brokeraugmenter.New(conf, fakeOnDemandBroker, nil, loggerFactory)
	Expect(err).NotTo(HaveOccurred())
	server := apiserver.New(
		conf,
		fakeBroker,
		componentName,
		loggerFactory,
		logger,
	)
	go apiserver.StartAndWait(conf, server, logger, stopServerChan)
	Eventually(loggerBuffer).Should(gbytes.Say("Listening on"))
}
