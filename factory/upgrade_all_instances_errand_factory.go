package factory

import (
	"time"

	"errors"
	"log"

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
	conf   config.UpgradeAllInstanceErrandConfig
	logger *log.Logger
}

func NewUpgradeAllInstancesErrandFactory(
	conf config.UpgradeAllInstanceErrandConfig,
	logger *log.Logger,
) *UpgradeAllInstancesErrandFactory {

	return &UpgradeAllInstancesErrandFactory{
		conf:   conf,
		logger: logger,
	}
}

func (f *UpgradeAllInstancesErrandFactory) Build() (*services.BrokerServices, *service.ServiceInstanceLister, int, error) {
	brokerServices, err := f.BrokerServices()
	if err != nil {
		return nil, nil, 0, err
	}

	serviceInstanceLister, err := f.ServiceInstanceLister()
	if err != nil {
		return nil, nil, 0, err
	}

	pollingInterval, err := f.PollingInterval()
	if err != nil {
		return nil, nil, 0, err
	}

	return brokerServices, serviceInstanceLister, pollingInterval, nil
}

func (f *UpgradeAllInstancesErrandFactory) BrokerServices() (*services.BrokerServices, error) {
	if f.conf.BrokerAPI.Authentication.Basic.Username == "" ||
		f.conf.BrokerAPI.Authentication.Basic.Password == "" ||
		f.conf.BrokerAPI.URL == "" {
		return &services.BrokerServices{}, errors.New("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		f.conf.BrokerAPI.Authentication.Basic.Username,
		f.conf.BrokerAPI.Authentication.Basic.Password,
	)

	return services.NewBrokerServices(
		herottp.New(herottp.Config{
			Timeout: 30 * time.Second,
		}),
		brokerBasicAuthHeaderBuilder,
		f.conf.BrokerAPI.URL,
		f.logger,
	), nil
}

func (f *UpgradeAllInstancesErrandFactory) ServiceInstanceLister() (*service.ServiceInstanceLister, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return &service.ServiceInstanceLister{},
			fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := f.conf.ServiceInstancesAPI.RootCACert
	certPool.AppendCertsFromPEM([]byte(cert))

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	manuallyConfigured := !strings.Contains(f.conf.ServiceInstancesAPI.URL, f.conf.BrokerAPI.URL)
	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		f.conf.ServiceInstancesAPI.Authentication.Basic.Username,
		f.conf.ServiceInstancesAPI.Authentication.Basic.Password,
	)
	return service.NewInstanceLister(
		httpClient,
		authHeaderBuilder,
		f.conf.ServiceInstancesAPI.URL,
		manuallyConfigured,
		f.logger,
	), nil
}

func (f *UpgradeAllInstancesErrandFactory) PollingInterval() (int, error) {
	if f.conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return f.conf.PollingInterval, nil
}
