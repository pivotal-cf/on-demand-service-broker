package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "register-broker", loggerfactory.Flags)
	logger := loggerFactory.New()

	configPath := parseConfigPathFlag(logger)
	configContents := readFileContent(configPath, logger)
	errandConfig := unmarshalConfig(configContents, logger)

	cfAuthenticator, err := errandConfig.CF.NewAuthHeaderBuilder(errandConfig.CF.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("Error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cf.New(errandConfig.CF.URL, cfAuthenticator, []byte(errandConfig.CF.TrustedCert), errandConfig.CF.DisableSSLCertVerification, logger)
	if err != nil {
		logger.Fatalf("Error creating Cloud Foundry client: %s", err)
	}

	registerBroker := registrar.RegisterBrokerRunner{
		Config:   errandConfig,
		CFClient: cfClient,
		Logger:   logger,
	}
	err = registerBroker.Run()
	if err != nil {
		logger.Fatal(err)
	}
}

func unmarshalConfig(configContents []byte, logger *log.Logger) config.RegisterBrokerErrandConfig {
	var conf config.RegisterBrokerErrandConfig
	err := yaml.Unmarshal(configContents, &conf)
	if err != nil {
		logger.Fatalf("error unmarshaling config file: %s", err.Error())
	}
	return conf
}

func parseConfigPathFlag(logger *log.Logger) string {
	var configPath string
	flag.StringVar(&configPath, "configPath", "", "path to register-broker config")
	flag.Parse()

	if configPath == "" {
		logger.Fatalln("-configPath must be given as argument")
	}
	return configPath
}

func readFileContent(filePath string, logger *log.Logger) []byte {
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		logger.Fatalf("error reading file -configPath: %s", err.Error())
	}
	return fileContents
}
