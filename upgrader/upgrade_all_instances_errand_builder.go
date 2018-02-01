package upgrader

import (
	"log"
	"time"

	"errors"

	"crypto/x509"
	"fmt"

	"strings"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/tools"
)

type Builder struct {
	BrokerServices        BrokerServices
	ServiceInstanceLister InstanceLister
	PollingInterval       time.Duration
	AttemptInterval       time.Duration
	AttemptLimit          int
	MaxInFlight           int
	Canaries              int
	Listener              Listener
	Sleeper               sleeper
}

func NewBuilder(
	conf config.UpgradeAllInstanceErrandConfig,
	logger *log.Logger,
) (*Builder, error) {

	brokerServices, err := brokerServices(conf, logger)
	if err != nil {
		return nil, err
	}

	instanceLister, err := serviceInstanceLister(conf, logger)
	if err != nil {
		return nil, err
	}

	pollingInterval, err := pollingInterval(conf)
	if err != nil {
		return nil, err
	}

	attemptInterval, err := attemptInterval(conf)
	if err != nil {
		return nil, err
	}

	attemptLimit, err := attemptLimit(conf)
	if err != nil {
		return nil, err
	}

	maxInFlight, err := maxInFlight(conf)
	if err != nil {
		return nil, err
	}

	canaries, err := canaries(conf)
	if err != nil {
		return nil, err
	}

	listener := NewLoggingListener(logger)

	b := &Builder{
		BrokerServices:        brokerServices,
		ServiceInstanceLister: instanceLister,
		PollingInterval:       pollingInterval,
		AttemptInterval:       attemptInterval,
		AttemptLimit:          attemptLimit,
		MaxInFlight:           maxInFlight,
		Canaries:              canaries,
		Listener:              listener,
		Sleeper:               &tools.RealSleeper{},
	}

	return b, nil
}

func brokerServices(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*services.BrokerServices, error) {
	if conf.BrokerAPI.Authentication.Basic.Username == "" ||
		conf.BrokerAPI.Authentication.Basic.Password == "" ||
		conf.BrokerAPI.URL == "" {
		return &services.BrokerServices{}, errors.New("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.BrokerAPI.Authentication.Basic.Username,
		conf.BrokerAPI.Authentication.Basic.Password,
	)

	return services.NewBrokerServices(
		herottp.New(herottp.Config{
			Timeout:    30 * time.Second,
			MaxRetries: 5,
		}),
		brokerBasicAuthHeaderBuilder,
		conf.BrokerAPI.URL,
		logger,
	), nil
}

func serviceInstanceLister(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*service.ServiceInstanceLister, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return &service.ServiceInstanceLister{},
			fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := conf.ServiceInstancesAPI.RootCACert
	certPool.AppendCertsFromPEM([]byte(cert))

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	manuallyConfigured := !strings.Contains(conf.ServiceInstancesAPI.URL, conf.BrokerAPI.URL)
	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.ServiceInstancesAPI.Authentication.Basic.Username,
		conf.ServiceInstancesAPI.Authentication.Basic.Password,
	)
	return service.NewInstanceLister(
		httpClient,
		authHeaderBuilder,
		conf.ServiceInstancesAPI.URL,
		manuallyConfigured,
		logger,
	), nil
}

func pollingInterval(conf config.UpgradeAllInstanceErrandConfig) (time.Duration, error) {
	if conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return time.Duration(conf.PollingInterval) * time.Second, nil
}

func attemptInterval(conf config.UpgradeAllInstanceErrandConfig) (time.Duration, error) {
	if conf.AttemptInterval <= 0 {
		return 0, errors.New("the attemptInterval must be greater than zero")
	}
	return time.Duration(conf.AttemptInterval) * time.Second, nil
}

func attemptLimit(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.AttemptLimit <= 0 {
		return 0, errors.New("the attempt limit must be greater than zero")
	}
	return conf.AttemptLimit, nil
}

func maxInFlight(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.MaxInFlight <= 0 {
		return 0, errors.New("the max in flight must be greater than zero")
	}
	return conf.MaxInFlight, nil
}

func canaries(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.Canaries < 0 {
		return 0, errors.New("the number of canaries cannot be negative")
	}
	return conf.Canaries, nil
}
