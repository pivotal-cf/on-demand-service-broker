package brokerinitiator

import (
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/telemetry"
	"log"
	"os"

	"github.com/pivotal-cf/on-demand-service-broker/broker/maintenanceinfo"
	"github.com/pivotal-cf/on-demand-service-broker/hasher"
	"github.com/pivotal-cf/on-demand-service-broker/service"

	credhub2 "code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

func Initiate(conf config.Config,
	brokerBoshClient broker.BoshClient,
	taskBoshClient task.BoshClient,
	cfClient broker.CloudFoundryClient,
	commandRunner serviceadapter.CommandRunner,
	stopServer chan os.Signal,
	loggerFactory *loggerfactory.LoggerFactory) {

	logger := loggerFactory.New()
	var err error
	startupChecks := buildStartupChecks(conf, cfClient, logger, brokerBoshClient)

	serviceAdapter := &serviceadapter.Client{
		ExternalBinPath: conf.ServiceAdapter.Path,
		CommandRunner:   commandRunner,
		UsingStdin:      conf.Broker.UsingStdin,
	}

	manifestGenerator := task.NewManifestGenerator(
		serviceAdapter,
		conf.ServiceCatalog,
		conf.ServiceDeployment.Stemcells,
		conf.ServiceDeployment.Releases,
	)
	odbSecrets := manifestsecrets.ODBSecrets{ServiceOfferingID: conf.ServiceCatalog.ID}
	boshCredhubStore := buildCredhubStore(conf, logger)
	preUpgradeChecker := task.NewPreUpgrade(manifestGenerator, taskBoshClient)

	deploymentManager := task.NewDeployer(taskBoshClient, manifestGenerator, odbSecrets, boshCredhubStore, preUpgradeChecker)
	deploymentManager.DisableBoshConfigs = conf.Broker.DisableBoshConfigs

	manifestSecretManager := manifestsecrets.BuildManager(conf.Broker.EnableSecureManifests, new(manifestsecrets.CredHubPathMatcher), boshCredhubStore)

	instanceLister, err := service.BuildInstanceLister(cfClient, conf.ServiceCatalog.ID, conf.ServiceInstancesAPI, logger)
	if err != nil {
		logger.Fatalf("error building instance lister: %s", err)
	}

	telemetryLogger := telemetry.Build(conf.Broker.EnableTelemetry, conf.ServiceCatalog, logger)

	var onDemandBroker apiserver.CombinedBroker
	onDemandBroker, err = broker.New(
		brokerBoshClient,
		cfClient,
		conf.ServiceCatalog,
		conf.Broker,
		startupChecks,
		serviceAdapter,
		deploymentManager,
		manifestSecretManager,
		instanceLister,
		&hasher.MapHasher{},
		loggerFactory,
		telemetryLogger,
		maintenanceinfo.Checker{},
	)
	if err != nil {
		logger.Fatalf("error starting broker: %s", err)
	}

	if conf.HasRuntimeCredHub() {
		onDemandBroker = wrapWithCredHubBroker(conf, logger, onDemandBroker, loggerFactory)
	}

	server := apiserver.New(
		conf,
		onDemandBroker,
		broker.ComponentName,
		loggerFactory,
		logger,
	)

	displayBanner(conf)

	telemetryLogger.LogInstances(instanceLister, "broker", "startup")

	if err := apiserver.StartAndWait(conf, server, logger, stopServer); err != nil {
		logger.Fatal(err)
	}
}

func wrapWithCredHubBroker(conf config.Config, logger *log.Logger, onDemandBroker apiserver.CombinedBroker, loggerFactory *loggerfactory.LoggerFactory) apiserver.CombinedBroker {
	err := network.NewHostWaiter().Wait(conf.CredHub.APIURL, 16, 10)
	if err != nil {
		logger.Fatalf("error connecting to runtime credhub: %s", err)
	}

	runtimeCredentialStore, err := credhub.Build(
		conf.CredHub.APIURL,
		credhub2.CaCerts(conf.CredHub.CaCert, conf.CredHub.InternalUAACaCert),
		credhub2.Auth(auth.UaaClientCredentials(conf.CredHub.ClientID, conf.CredHub.ClientSecret)),
	)

	if err != nil {
		logger.Fatalf("error creating runtime credhub client: %s", err)
	}
	return credhubbroker.New(onDemandBroker, runtimeCredentialStore, conf.ServiceCatalog.Name, loggerFactory)
}

func buildCredhubStore(conf config.Config, logger *log.Logger) *credhub.Store {
	var boshCredhubStore *credhub.Store
	var err error
	if conf.Broker.EnableSecureManifests {
		boshCredhubStore, err = credhub.Build(
			conf.BoshCredhub.URL,
			credhub2.Auth(auth.UaaClientCredentials(
				conf.BoshCredhub.Authentication.UAA.ClientCredentials.ID,
				conf.BoshCredhub.Authentication.UAA.ClientCredentials.Secret,
			)),
			credhub2.CaCerts(conf.BoshCredhub.RootCACert, conf.Bosh.TrustedCert),
		)
		if err != nil {
			logger.Fatalf("error starting broker: %s", err)
		}
	}
	return boshCredhubStore
}

func buildStartupChecks(conf config.Config, cfClient broker.CloudFoundryClient, logger *log.Logger, boshClient broker.BoshClient) []broker.StartupChecker {
	var startupChecks []broker.StartupChecker
	if !conf.Broker.DisableCFStartupChecks {
		startupChecks = append(
			startupChecks,
			startupchecker.NewCFAPIVersionChecker(cfClient, broker.MinimumCFVersion, logger),
			startupchecker.NewCFPlanConsistencyChecker(cfClient, conf.ServiceCatalog, logger),
		)

	}
	boshInfo, err := boshClient.GetInfo(logger)
	if err != nil {
		logger.Fatalf("error starting broker: %s", err)
	}
	startupChecks = append(startupChecks,
		startupchecker.NewBOSHDirectorVersionChecker(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
			boshInfo,
			conf,
		),
		startupchecker.NewBOSHAuthChecker(boshClient, logger),
	)
	return startupChecks
}

func displayBanner(conf config.Config) {
	if conf.Broker.StartUpBanner {
		fmt.Println(`
					                  .//\
					        \\      .+ssso/\     \\
					      \---.\  .+ssssssso/.  \----\         ____     ______     ______
					    .--------+ssssssssssso+--------\      / __ \   (_  __ \   (_   _ \
					  .-------:+ssssssssssssssss+--------\   / /  \ \    ) ) \ \    ) (_) )
					 -------./ssssssssssssssssssss:.------- ( ()  () )  ( (   ) )   \   _/
					  \--------+ssssssssssssssso+--------/  ( ()  () )   ) )  ) )   /  _ \
					    \-------.+osssssssssso/.-------/     \ \__/ /   / /__/ /   _) (_) )
					      \---./  ./osssssso/   \.---/        \____/   (______/   (______/
					        \/      \/osso/       \/
					                  \/:/
													`)
	}
}
