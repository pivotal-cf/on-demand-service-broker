// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Broker            Broker
	Bosh              Bosh
	CF                CF
	ServiceAdapter    ServiceAdapter    `yaml:"service_adapter"`
	ServiceDeployment ServiceDeployment `yaml:"service_deployment"`
	ServiceCatalog    ServiceOffering   `yaml:"service_catalog"`
}

func (c Config) Validate() error {
	if err := c.Broker.Validate(); err != nil {
		return err
	}

	if err := c.Bosh.Validate(); err != nil {
		return err
	}

	if err := c.CF.Validate(); err != nil {
		return err
	}

	if err := checkIsExecutableFile(c.ServiceAdapter.Path); err != nil {
		return fmt.Errorf("checking for executable service adapter file: %s", err)
	}

	if err := c.ServiceDeployment.Validate(); err != nil {
		return err
	}

	return nil
}

type Broker struct {
	Port                       int
	Username                   string
	Password                   string
	DisableSSLCertVerification bool `yaml:"disable_ssl_cert_verification"`
	StartUpBanner              bool `yaml:"startup_banner"`
	ShutdownTimeoutSecs        int  `yaml:"shutdown_timeout_in_seconds"`
}

func (b Broker) Validate() error {
	if b.Port == 0 {
		return errors.New("broker.port can't be empty")
	}
	if b.Username == "" {
		return errors.New("broker.username can't be empty")
	}
	if b.Password == "" {
		return errors.New("broker.password can't be empty")
	}

	return nil
}

type ServiceDeployment struct {
	Releases serviceadapter.ServiceReleases
	Stemcell serviceadapter.Stemcell
}

func (s ServiceDeployment) Validate() error {
	for _, release := range s.Releases {
		if err := assertVersion(release.Version); err != nil {
			return err
		}
	}

	if err := assertVersion(s.Stemcell.Version); err != nil {
		return err
	}

	return nil
}

func assertVersion(version string) error {
	if strings.HasSuffix(version, "latest") {
		return errors.New("You must configure the exact release and stemcell versions in broker.service_deployment. ODB requires exact versions to detect pending changes as part of the 'cf update-service' workflow. For example, latest and 3112.latest are not supported.")
	}
	return nil
}

type Bosh struct {
	URL            string
	TrustedCert    string `yaml:"root_ca_cert"`
	Authentication BOSHAuthentication
}

type BOSHAuthentication struct {
	Basic UserCredentials
	UAA   BOSHUAAAuthentication
}

type CF struct {
	URL            string
	TrustedCert    string `yaml:"root_ca_cert"`
	Authentication UAAAuthentication
}

type AuthHeaderBuilder interface {
	Build(logger *log.Logger) (string, error)
}

func (cf CF) NewAuthHeaderBuilder(disableSSLCertVerification bool) (AuthHeaderBuilder, error) {
	if cf.Authentication.ClientCredentials.IsSet() {
		return authorizationheader.NewClientTokenAuthHeaderBuilder(
			cf.Authentication.URL,
			cf.Authentication.ClientCredentials.ID,
			cf.Authentication.ClientCredentials.Secret,
			disableSSLCertVerification,
			[]byte(cf.TrustedCert),
		)
	} else {
		return authorizationheader.NewUserTokenAuthHeaderBuilder(
			cf.Authentication.URL,
			"cf",
			"",
			cf.Authentication.UserCredentials.Username,
			cf.Authentication.UserCredentials.Password,
			disableSSLCertVerification,
			[]byte(cf.TrustedCert),
		)
	}
}

type UserCredentials struct {
	Username string
	Password string
}

func (c UserCredentials) IsSet() bool {
	return c != UserCredentials{}
}

type ClientCredentials struct {
	ID     string `yaml:"client_id"`
	Secret string `yaml:"secret"`
}

type UAAAuthentication struct {
	URL               string            `yaml:"url"`
	ClientCredentials ClientCredentials `yaml:"client_credentials"`
	UserCredentials   UserCredentials   `yaml:"user_credentials"`
}

type BOSHUAAAuthentication struct {
	UAAURL string `yaml:"url"`
	ID     string `yaml:"client_id"`
	Secret string `yaml:"client_secret"`
}

func (a UAAAuthentication) IsSet() bool {
	return a != UAAAuthentication{}
}

func (b Bosh) Validate() error {
	if b.URL == "" {
		return fmt.Errorf("Must specify bosh url")
	}
	return b.Authentication.Validate()
}

func (cf CF) Validate() error {
	if cf.URL == "" {
		return fmt.Errorf("Must specify CF url")
	}
	return cf.Authentication.Validate()
}

func (a UAAAuthentication) Validate() error {
	urlIsSet := a.URL != ""
	clientIsSet := a.ClientCredentials.IsSet()
	basicSet := a.UserCredentials.IsSet()

	var err error

	if !urlIsSet && !clientIsSet && !basicSet {
		return fmt.Errorf("Must specify UAA authentication")
	} else if !urlIsSet {
		return fmt.Errorf("Must specify UAA url")
	} else if !clientIsSet && !basicSet {
		return fmt.Errorf("Must specify UAA credentials")
	} else if clientIsSet && basicSet {
		err = fmt.Errorf("Cannot specify both client and user credentials for UAA authentication")
	} else if clientIsSet {
		err = validateNoFieldsEmptyString(a.ClientCredentials, "client_credentials")
	} else if basicSet {
		err = validateNoFieldsEmptyString(a.UserCredentials, "user_credentials")
	}

	return err
}

func (cc BOSHUAAAuthentication) IsSet() bool {
	return cc != BOSHUAAAuthentication{}
}

func (cc ClientCredentials) IsSet() bool {
	return cc != ClientCredentials{}
}

func (a BOSHAuthentication) Validate() error {
	uaaSet := a.UAA.IsSet()
	basicSet := a.Basic.IsSet()

	var err error

	if !uaaSet && !basicSet {
		return fmt.Errorf("Must specify bosh authentication")
	} else if uaaSet && basicSet {
		err = fmt.Errorf("Cannot specify both basic and UAA for BOSH authentication")
	} else if uaaSet {
		err = validateNoFieldsEmptyString(a.UAA, "uaa")
	} else if basicSet {
		err = validateNoFieldsEmptyString(a.Basic, "basic")
	}

	return err
}

type ServiceAdapter struct {
	Path string
}

func Parse(configFilePath string) (Config, error) {
	configFileBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := yaml.Unmarshal(configFileBytes, &config); err != nil {
		return Config{}, err
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func validateNoFieldsEmptyString(obj interface{}, objName string) error {
	bVal := reflect.ValueOf(obj)
	bType := reflect.TypeOf(obj)
	for i := 0; i < bVal.NumField(); i++ {
		fieldVal := bVal.Field(i).String()

		if fieldVal == "" {
			fieldName := bType.Field(i).Name
			return fmt.Errorf("%s.%s can't be empty", objName, strings.ToLower(fieldName))
		}
	}

	return nil
}

type ServiceOffering struct {
	ID               string
	Name             string `yaml:"service_name"`
	Description      string `yaml:"service_description"`
	Bindable         bool
	PlanUpdatable    bool     `yaml:"plan_updatable"`
	Requires         []string `yaml:"requires,omitempty"`
	Metadata         ServiceMetadata
	DashboardClient  *DashboardClient `yaml:"dashboard_client,omitempty"`
	Tags             []string
	GlobalProperties serviceadapter.Properties `yaml:"global_properties"`
	GlobalQuotas     Quotas                    `yaml:"global_quotas"`
	Plans            Plans
}

func (s ServiceOffering) FindPlanByID(id string) (Plan, bool) {
	return s.Plans.FindByID(id)
}

func (s ServiceOffering) HasLifecycleErrands() bool {
	for _, plan := range s.Plans {
		if plan.LifecycleErrands != nil {
			if plan.LifecycleErrands.PostDeploy != "" || plan.LifecycleErrands.PreDelete != "" {
				return true
			}
		}
	}

	return false
}

type Plans []Plan

func (p Plans) FindByID(id string) (Plan, bool) {
	for _, plan := range p {
		if plan.ID == id {
			return plan, true
		}
	}
	return Plan{}, false
}

type Plan struct {
	ID               string `yaml:"plan_id"`
	Name             string
	Free             *bool
	Bindable         *bool
	Description      string
	Metadata         PlanMetadata
	Quotas           Quotas `yaml:"quotas,omitempty"`
	Properties       serviceadapter.Properties
	InstanceGroups   []serviceadapter.InstanceGroup `yaml:"instance_groups,omitempty"`
	Update           *serviceadapter.Update         `yaml:"update,omitempty"`
	LifecycleErrands *LifecycleErrands              `yaml:"lifecycle_errands,omitempty"`
}

func (p Plan) AdapterPlan(globalProperties serviceadapter.Properties) serviceadapter.Plan {
	return serviceadapter.Plan{
		Properties:     mergeProperties(p.Properties, globalProperties),
		InstanceGroups: p.InstanceGroups,
		Update:         p.Update,
	}
}

func mergeProperties(planProperties, globalProperties serviceadapter.Properties) serviceadapter.Properties {
	properties := serviceadapter.Properties{}
	for k, v := range globalProperties {
		properties[k] = v
	}
	for k, v := range planProperties {
		properties[k] = v
	}
	return properties
}

func (p Plan) PostDeployErrand() string {
	if p.LifecycleErrands == nil {
		return ""
	}

	return p.LifecycleErrands.PostDeploy
}

func (p Plan) PreDeleteErrand() string {
	if p.LifecycleErrands == nil {
		return ""
	}

	return p.LifecycleErrands.PreDelete
}

type LifecycleErrands struct {
	PostDeploy string `yaml:"post_deploy"`
	PreDelete  string `yaml:"pre_delete"`
}

type PlanMetadata struct {
	DisplayName string     `yaml:"display_name"`
	Bullets     []string   `yaml:"bullets,omitempty"`
	Costs       []PlanCost `yaml:"costs"`
}

type PlanCost struct {
	Amount map[string]float64 `yaml:"amount"`
	Unit   string             `yaml:"unit"`
}

type ServiceMetadata struct {
	DisplayName         string `yaml:"display_name"`
	ImageURL            string `yaml:"image_url"`
	LongDescription     string `yaml:"long_description"`
	ProviderDisplayName string `yaml:"provider_display_name"`
	DocumentationURL    string `yaml:"documentation_url"`
	SupportURL          string `yaml:"support_url"`
}

type DashboardClient struct {
	ID          string
	Secret      string
	RedirectUri string `yaml:"redirect_uri"`
}

type Quotas struct {
	ServiceInstanceLimit *int `yaml:"service_instance_limit,omitempty"`
}
