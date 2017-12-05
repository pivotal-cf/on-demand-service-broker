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
)

type Builder struct {
	BrokerServices        BrokerServices
	ServiceInstanceLister InstanceLister
	PollingInterval       time.Duration
	AttemptLimit          int
	Listener              Listener
}

func NewBuilder(
	conf config.UpgradeAllInstanceErrandConfig,
	logger *log.Logger,
) (*Builder, error) {

	brokerServices, err := Broker(conf, logger)
	if err != nil {
		return nil, err
	}

	instanceLister, err := ServiceInstanceLister(conf, logger)
	if err != nil {
		return nil, err
	}

	pollingInterval, err := PollingInterval(conf, logger)
	if err != nil {
		return nil, err
	}

	attemptLimit, err := AttemptLimit(conf, logger)
	if err != nil {
		return nil, err
	}

	listener := NewLoggingListener(logger)

	return &Builder{
		brokerServices,
		instanceLister,
		pollingInterval,
		attemptLimit,
		listener,
	}, nil
}

func Broker(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*services.BrokerServices, error) {
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
			Timeout: 30 * time.Second,
		}),
		brokerBasicAuthHeaderBuilder,
		conf.BrokerAPI.URL,
		logger,
	), nil
}

func ServiceInstanceLister(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*service.ServiceInstanceLister, error) {
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

func PollingInterval(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (time.Duration, error) {
	if conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return time.Duration(conf.PollingInterval) * time.Second, nil
}

func AttemptLimit(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (int, error) {
	if conf.AttemptLimit <= 0 {
		return 0, errors.New("the attempt limit must be greater than zero")
	}
	return conf.AttemptLimit, nil
}
