package brokerinitiator

import (
	"fmt"
	"os"

	credhub2 "github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
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

func Start(conf config.Config,
	boshClient *boshdirector.Client,
	cfClient broker.CloudFoundryClient,
	commandRunner serviceadapter.CommandRunner,
	stopServer chan os.Signal,
	loggerFactory *loggerfactory.LoggerFactory) {
	logger := loggerFactory.New()
	var err error
	var startupChecks []broker.StartupChecker
	if !conf.Broker.DisableCFStartupChecks {
		startupChecks = append(
			startupChecks,
			startupchecker.NewCFAPIVersionChecker(cfClient, broker.MinimumCFVersion, logger),
			startupchecker.NewCFPlanConsistencyChecker(cfClient, conf.ServiceCatalog, logger),
		)

	}

	serviceAdapter := &serviceadapter.Client{
		ExternalBinPath: conf.ServiceAdapter.Path,
		CommandRunner:   commandRunner,
		UsingStdin:      conf.Broker.UsingStdin,
	}

	startupChecks = append(startupChecks,
		startupchecker.NewBOSHDirectorVersionChecker(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
			broker.MinimumSemverVersionForBindingWithDNS,
			boshClient.BoshInfo,
			conf,
		),
		startupchecker.NewBOSHAuthChecker(boshClient, logger),
	)

	var boshCredhubStore *credhub.Store
	matcher := new(manifestsecrets.CredHubPathMatcher)
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

	manifestGenerator := task.NewManifestGenerator(
		serviceAdapter,
		conf.ServiceCatalog,
		conf.ServiceDeployment.Stemcell,
		conf.ServiceDeployment.Releases,
	)
	odbSecrets := manifestsecrets.ODBSecrets{ServiceOfferingID: conf.ServiceCatalog.ID}
	deploymentManager := task.NewDeployer(boshClient, manifestGenerator, odbSecrets, boshCredhubStore)

	manifestSecretManager := manifestsecrets.BuildManager(conf.Broker.EnableSecureManifests, matcher, boshCredhubStore)

	var onDemandBroker apiserver.CombinedBroker
	onDemandBroker, err = broker.New(
		boshClient,
		cfClient,
		conf.ServiceCatalog,
		conf.Broker,
		startupChecks,
		serviceAdapter,
		deploymentManager,
		manifestSecretManager,
		loggerFactory,
	)
	if err != nil {
		logger.Fatalf("error starting broker: %s", err)
	}

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

	if conf.HasCredHub() {
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
		onDemandBroker = credhubbroker.New(onDemandBroker, runtimeCredentialStore, conf.ServiceCatalog.Name, loggerFactory)
	}

	server := apiserver.New(
		conf,
		onDemandBroker,
		broker.ComponentName,
		loggerFactory,
		logger,
	)

	apiserver.StartAndWait(conf, server, logger, stopServer)
}
