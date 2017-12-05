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

type UpgradeAllInstancesErrandFactory struct {
	Conf   config.UpgradeAllInstanceErrandConfig
	Logger *log.Logger
}

func New(
	conf config.UpgradeAllInstanceErrandConfig,
	logger *log.Logger,
) (*upgrader, error) {

	f := UpgradeAllInstancesErrandFactory{
		Conf:   conf,
		Logger: logger,
	}

	brokerServices, err := f.BrokerServices()
	if err != nil {
		return nil, err
	}

	instanceLister, err := f.ServiceInstanceLister()
	if err != nil {
		return nil, err
	}

	pollingInterval, err := f.PollingInterval()
	if err != nil {
		return nil, err
	}

	attemptLimit, err := f.AttemptLimit()
	if err != nil {
		return nil, err
	}

	listener := NewLoggingListener(logger)

	return NewUpgrader(
		brokerServices,
		instanceLister,
		pollingInterval,
		attemptLimit,
		listener,
	), nil
}

func (f *UpgradeAllInstancesErrandFactory) BrokerServices() (*services.BrokerServices, error) {
	if f.Conf.BrokerAPI.Authentication.Basic.Username == "" ||
		f.Conf.BrokerAPI.Authentication.Basic.Password == "" ||
		f.Conf.BrokerAPI.URL == "" {
		return &services.BrokerServices{}, errors.New("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		f.Conf.BrokerAPI.Authentication.Basic.Username,
		f.Conf.BrokerAPI.Authentication.Basic.Password,
	)

	return services.NewBrokerServices(
		herottp.New(herottp.Config{
			Timeout: 30 * time.Second,
		}),
		brokerBasicAuthHeaderBuilder,
		f.Conf.BrokerAPI.URL,
		f.Logger,
	), nil
}

func (f *UpgradeAllInstancesErrandFactory) ServiceInstanceLister() (*service.ServiceInstanceLister, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return &service.ServiceInstanceLister{},
			fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := f.Conf.ServiceInstancesAPI.RootCACert
	certPool.AppendCertsFromPEM([]byte(cert))

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	manuallyConfigured := !strings.Contains(f.Conf.ServiceInstancesAPI.URL, f.Conf.BrokerAPI.URL)
	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		f.Conf.ServiceInstancesAPI.Authentication.Basic.Username,
		f.Conf.ServiceInstancesAPI.Authentication.Basic.Password,
	)
	return service.NewInstanceLister(
		httpClient,
		authHeaderBuilder,
		f.Conf.ServiceInstancesAPI.URL,
		manuallyConfigured,
		f.Logger,
	), nil
}

func (f *UpgradeAllInstancesErrandFactory) PollingInterval() (time.Duration, error) {
	if f.Conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return time.Duration(f.Conf.PollingInterval) * time.Second, nil
}

func (f *UpgradeAllInstancesErrandFactory) AttemptLimit() (int, error) {
	if f.Conf.AttemptLimit <= 0 {
		return 0, errors.New("the attempt limit must be greater than zero")
	}
	return f.Conf.AttemptLimit, nil
}
