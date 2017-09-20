package main

import (
	"os"

	"gopkg.in/yaml.v2"

	"flag"

	"io/ioutil"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deregistrar"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "deregister-broker", loggerfactory.Flags)
	logger := loggerFactory.New()

	brokerName := flag.String("brokerName", "", "broker name")
	configFilePath := flag.String("configFilePath", "", "path to config file")
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

	var config deregistrar.Config
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

	deregistrarTool := deregistrar.New(cfClient, logger)
	err = deregistrarTool.Deregister(*brokerName)
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Println("FINISHED DEREGISTER BROKER")
}
