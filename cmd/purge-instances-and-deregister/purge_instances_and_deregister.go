package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/purger"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"gopkg.in/yaml.v2"
)

type realSleeper struct{}

func (c realSleeper) Sleep(t time.Duration) { time.Sleep(t) }

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "purge-instances-and-deregister-broker", loggerfactory.Flags)
	logger := loggerFactory.New()

	configFilePath := flag.String("configFilePath", "", "path to config file")
	brokerName := flag.String("brokerName", "", "broker name")
	flag.Parse()

	if *brokerName == "" {
		logger.Fatal("Missing argument -brokerName")
	}

	if *configFilePath == "" {
		logger.Fatal("Missing argument -configFilePath")
	}

	rawConfig, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		logger.Fatalf("Error reading config file: %s", err)
	}

	var config deleter.Config
	err = yaml.Unmarshal(rawConfig, &config)
	if err != nil {
		logger.Fatalf("Invalid config file: %s", err)
	}

	cfAuthenticator, err := config.CF.NewAuthHeaderBuilder(config.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("Error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cf.New(
		config.CF.URL,
		cfAuthenticator,
		[]byte(config.CF.TrustedCert),
		config.DisableSSLCertVerification,
	)
	if err != nil {
		logger.Fatalf("Error creating Cloud Foundry client: %s", err)
	}

	clock := realSleeper{}

	deleteTool := deleter.New(cfClient, clock, config.PollingInitialOffset, config.PollingInterval, logger)

	registrarTool := registrar.New(cfClient, logger)

	purgerTool := purger.New(deleteTool, registrarTool, cfClient, logger)

	err = purgerTool.DeleteInstancesAndDeregister(config.ServiceCatalog.ID, *brokerName)
	if err != nil {
		logger.Fatalf(err.Error())
	}
	fmt.Println("FINISHED PURGE INSTANCES AND DEREGISTER BROKER")
}
