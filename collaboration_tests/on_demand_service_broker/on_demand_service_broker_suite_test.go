package on_demand_service_broker_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	"os"
	"syscall"
	"testing"

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
	serverPort     = 1337
	serviceName    = "service-name"
	brokerUsername = "username"
	brokerPassword = "password"
)

var (
	stopServer         chan os.Signal
	serverURL          = fmt.Sprintf("localhost:%d", serverPort)
	fakeServiceAdapter *fakes.FakeServiceAdapterClient
	fakeBoshClient     *fakes.FakeBoshClient
	fakeCfClient       *fakes.FakeCloudFoundryClient
	fakeDeployer       *fakes.FakeDeployer
	loggerBuffer       *gbytes.Buffer
)

var _ = BeforeEach(func() {
	fakeBoshClient = new(fakes.FakeBoshClient)
	fakeServiceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeCfClient = new(fakes.FakeCloudFoundryClient)
	fakeDeployer = new(fakes.FakeDeployer)

	stopServer = make(chan os.Signal, 1)
})

var _ = AfterEach(func() {
	stopServer <- syscall.SIGTERM
})

func StartServer(conf config.Config) {
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
	go apiserver.StartAndWait(conf, server, logger, stopServer)
}
