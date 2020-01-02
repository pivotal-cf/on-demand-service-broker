package brokerinitiator_test

import (
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/brokerinitiator"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	serviceAdapterFakes "github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	tasksFakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Start", func() {
	var (
		stopServer             chan os.Signal
		brokerConfig           config.Config
		logBuffer              *gbytes.Buffer
		loggerFactory          *loggerfactory.LoggerFactory
		fakeBoshClient         *fakes.FakeBoshClient
		fakeCloudFoundryClient *fakes.FakeCloudFoundryClient
	)

	BeforeEach(func() {
		stopServer = make(chan os.Signal, 1)
		brokerConfig = config.Config{
			Broker: config.Broker{
				Port:                       8080,
				Username:                   "username",
				Password:                   "password",
				DisableSSLCertVerification: true,
				ShutdownTimeoutSecs:        10,
				UsingStdin:                 true,
				DisableCFStartupChecks:     true,
				DisableBoshConfigs:         true,
				EnableTelemetry:            true,
			},
			Bosh: config.Bosh{
				URL:         "some-url",
				TrustedCert: "some-cert",
				Authentication: config.Authentication{
					Basic: config.UserCredentials{
						Username: "some-username",
						Password: "some-password",
					},
				},
			},
			ServiceCatalog: config.ServiceOffering{
				ID:   "service-id",
				Name: "service-name",
				Plans: []config.Plan{
					{ID: "a-plan-id", Name: "plan-name"},
					{ID: "another-plan-id", Name: "another-plan-name"},
				},
			},
			ServiceAdapter: config.ServiceAdapter{
				Path: "test_assets/executable.sh",
			},
			ServiceDeployment: config.ServiceDeployment{
				Releases: serviceadapter.ServiceReleases{{
					Name:    "some-name",
					Version: "some-version",
					Jobs:    []string{"some-job"},
				}},
				Stemcells: []serviceadapter.Stemcell{{OS: "ubuntu-trusty", Version: "1234"}},
			},
		}

		logBuffer = gbytes.NewBuffer()
		loggerFactory = loggerfactory.New(logBuffer, "startup-test", loggerfactory.Flags)

		fakeBoshClient = new(fakes.FakeBoshClient)
		fakeBoshClient.GetInfoReturns(boshdirector.Info{Version: "3262.0"}, nil)

		fakeCloudFoundryClient = new(fakes.FakeCloudFoundryClient)
	})

	AfterEach(func() {
		stopServer <- os.Kill
	})

	It("logs telemetry data when telemetry is enabled", func() {
		fakeCloudFoundryClient.GetServiceInstancesReturns([]cf.Instance{
			{GUID: "123", PlanUniqueID: "a-plan-id"},
			{GUID: "321", PlanUniqueID: "a-plan-id"},
		}, nil)

		go brokerinitiator.Initiate(
			brokerConfig,
			fakeBoshClient,
			new(tasksFakes.FakeBoshClient),
			fakeCloudFoundryClient,
			new(serviceAdapterFakes.FakeCommandRunner),
			stopServer,
			loggerFactory,
		)

		Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"service-instances":{"total":2},"event":{"item":"broker","operation":"startup"}}`, brokerConfig.ServiceCatalog.Name)))
		Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"service-instances-per-plan":{"plan-id":"a-plan-id","total":2},"event":{"item":"broker","operation":"startup"}}`, brokerConfig.ServiceCatalog.Name)))
		Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`"telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"service-instances-per-plan":{"plan-id":"another-plan-id","total":0},"event":{"item":"broker","operation":"startup"}}`, brokerConfig.ServiceCatalog.Name)))
	})

	It("doesn't log telemetry data when telemetry is not enabled", func() {
		brokerConfig.Broker.EnableTelemetry = false

		go brokerinitiator.Initiate(
			brokerConfig,
			fakeBoshClient,
			new(tasksFakes.FakeBoshClient),
			fakeCloudFoundryClient,
			new(serviceAdapterFakes.FakeCommandRunner),
			stopServer,
			loggerFactory,
		)

		Consistently(logBuffer).ShouldNot(gbytes.Say("telemetry-source"))
	})

	It("logs error when cannot query list of instances", func() {
		fakeCloudFoundryClient.GetServiceInstancesReturns([]cf.Instance{
			{GUID: "123", PlanUniqueID: "plan-id"},
			{GUID: "321", PlanUniqueID: "plan-id"},
		}, errors.New("nope"))

		go brokerinitiator.Initiate(
			brokerConfig,
			fakeBoshClient,
			new(tasksFakes.FakeBoshClient),
			fakeCloudFoundryClient,
			new(serviceAdapterFakes.FakeCommandRunner),
			stopServer,
			loggerFactory,
		)

		Eventually(logBuffer).Should(gbytes.Say("Failed to query list of instances for telemetry"))
		Consistently(logBuffer).ShouldNot(gbytes.Say("telemetry-source"))
	})
})
