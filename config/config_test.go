// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package config_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Broker Config", func() {
	var (
		configFileName string
		conf           config.Config
		parseErr       error
	)

	JustBeforeEach(func() {
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		configFilePath := filepath.Join(cwd, "test_assets", configFileName)
		conf, parseErr = config.Parse(configFilePath)
	})

	Context("when the config file is valid YAML", func() {
		Context("and the config is good", func() {
			BeforeEach(func() {
				configFileName = "good_config.yml"
			})

			It("returns config object", func() {
				instanceLimit := 1
				expected := config.Config{
					Broker: config.Broker{
						Port:                       8080,
						Username:                   "username",
						Password:                   "password",
						DisableSSLCertVerification: true,
						StartUpBanner:              false,
						ExposeOperationalErrors:    false,
						EnablePlanSchemas:          false,
						ShutdownTimeoutSecs:        10,
						UsingStdin:                 true,
					},
					Bosh: config.Bosh{
						URL:         "some-url",
						TrustedCert: "some-cert",
						Authentication: config.Authentication{
							Basic: config.UserCredentials{
								Username: "some-username",
								Password: "some-password",
							},
						},
					},
					CF: config.CF{
						URL:         "some-cf-url",
						TrustedCert: "some-cf-cert",
						Authentication: config.Authentication{
							UAA: config.UAAAuthentication{
								URL: "a-uaa-url",
								UserCredentials: config.UserCredentials{
									Username: "some-cf-username",
									Password: "some-cf-password",
								},
							},
						},
					},
					ServiceInstancesAPI: config.ServiceInstancesAPI{
						URL:        "some-si-api-url",
						RootCACert: "some-cert",
						Authentication: config.Authentication{
							Basic: config.UserCredentials{
								Username: "si-api-username",
								Password: "si-api-password",
							},
						},
					},
					ServiceAdapter: config.ServiceAdapter{
						Path: "test_assets/executable.sh",
					},
					ServiceDeployment: config.ServiceDeployment{
						Releases: serviceadapter.ServiceReleases{{
							Name:    "some-name",
							Version: "some-version",
							Jobs:    []string{"some-job"},
						}},
						Stemcell: serviceadapter.Stemcell{OS: "ubuntu-trusty", Version: "1234"},
					},
					ServiceCatalog: config.ServiceOffering{
						ID:            "some-id",
						Name:          "some-marketplace-name",
						Description:   "some-description",
						Bindable:      true,
						PlanUpdatable: true,
						Metadata: config.ServiceMetadata{
							DisplayName:         "some-service-display-name",
							ImageURL:            "http://test.jpg",
							LongDescription:     "Some description",
							ProviderDisplayName: "some name",
							DocumentationURL:    "some url",
							SupportURL:          "some url",
							Shareable:           true,
						},
						DashboardClient: &config.DashboardClient{
							ID:          "client-id-1",
							Secret:      "secret-1",
							RedirectUri: "https://dashboard.url",
						},
						Tags:             []string{"some-tag", "some-other-tag"},
						GlobalProperties: serviceadapter.Properties{"global_foo": "global_bar"},
						Plans: []config.Plan{
							{
								ID:          "some-dedicated-plan-id",
								Name:        "some-dedicated-name",
								Description: "I'm a dedicated plan",
								Free:        booleanPointer(true),
								Metadata: config.PlanMetadata{
									DisplayName: "Dedicated-Cluster",
									Bullets:     []string{"bullet one", "bullet two", "bullet three"},
									Costs: []config.PlanCost{
										{Amount: map[string]float64{"usd": 99.0, "eur": 49.0}, Unit: "MONTHLY"},
										{Amount: map[string]float64{"usd": 0.99, "eur": 0.49}, Unit: "1GB of messages over 20GB"},
									},
								},
								Quotas: config.Quotas{
									ServiceInstanceLimit: &instanceLimit,
								},
								Properties: serviceadapter.Properties{
									"persistence": true,
								},
								LifecycleErrands: &serviceadapter.LifecycleErrands{
									PostDeploy: []serviceadapter.Errand{{
										Name:      "health-check",
										Instances: []string{"redis-errand/0", "redis-errand/1"},
									}},
									PreDelete: []serviceadapter.Errand{{
										Name:      "cleanup",
										Instances: []string{"redis-errand/0"},
									}},
								},
								InstanceGroups: []serviceadapter.InstanceGroup{
									{
										Name:               "redis-server",
										VMType:             "some-vm",
										PersistentDiskType: "some-disk",
										Instances:          34,
										Networks:           []string{"net1", "net2"},
										VMExtensions:       []string{},
									},
									{
										Name:         "redis-server-2",
										VMType:       "some-vm-2",
										Instances:    3,
										Networks:     []string{"net4", "net5"},
										VMExtensions: []string{},
									},
									{
										Name:         "redis-errand",
										VMType:       "some-vm-3",
										Instances:    2,
										Networks:     []string{"net5", "net6"},
										Lifecycle:    "errand",
										VMExtensions: []string{},
									},
								},
								Update: &serviceadapter.Update{
									Canaries:        1,
									MaxInFlight:     2,
									CanaryWatchTime: "1000-30000",
									UpdateWatchTime: "1000-30000",
									Serial:          booleanPointer(false),
								},
							},
						},
					},
				}

				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf).To(Equal(expected))
			})
		})

		Context("and the config has optional global resource quotas", func() {
			BeforeEach(func() {
				configFileName = "good_config_with_global_resource_quotas.yml"
			})

			It("returns config object", func() {
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf.ServiceCatalog.GlobalQuotas.ResourceLimits["ips"]).To(Equal(1))
				Expect(conf.ServiceCatalog.Plans[0].ResourceCosts["ips"]).To(Equal(1))
			})
		})

		Context("and the config has the expose_operational_errors flag", func() {
			BeforeEach(func() {
				configFileName = "good_config_with_optional_fields.yml"
			})

			It("returns config object", func() {
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf.Broker.ExposeOperationalErrors).To(BeTrue())
				Expect(conf.Broker.EnablePlanSchemas).To(BeTrue())
				Expect(conf.Broker.EnableSecureManifests).To(BeTrue())
				Expect(conf.BoshCredhub.URL).To(Equal("https://bosh-credhub:8844/api/"))
				Expect(conf.BoshCredhub.RootCACert).To(Equal("CERT"))
				Expect(conf.BoshCredhub.Authentication.UAA.ClientCredentials.ID).To(Equal("credhub_id"))
				Expect(conf.BoshCredhub.Authentication.UAA.ClientCredentials.Secret).To(Equal("credhub_secret"))
				Expect(conf.ServiceCatalog.Plans[0].Metadata.AdditionalMetadata).To(Equal(map[string]interface{}{
					"workers": 42,
				}))
				Expect(conf.ServiceCatalog.Metadata.AdditionalMetadata).To(Equal(map[string]interface{}{
					"managers": 137,
				}))
			})

		})

		Context("and the config includes the optional plan property binding_with_dns", func() {
			BeforeEach(func() {
				configFileName = "good_config_with_binding_dns_list.yml"
			})

			It("returns a config object with binding dns properties", func() {
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf.ServiceCatalog.Plans[0].BindingWithDNS).To(ConsistOf([]config.BindingDNS{
					{Name: "leader", LinkProvider: "leader_link", InstanceGroup: "redis-server"},
					{Name: "follower", LinkProvider: "follower_link", InstanceGroup: "redis-server-2"},
				}))
			})
		})

		Context("and the broker password contains escaped special characters", func() {
			BeforeEach(func() {
				configFileName = "escaped_config.yml"
			})

			It("returns config with the broker password", func() {
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf.Broker.Password).To(Equal(`%te'"st:%$!`))
			})
		})

		Context("when service catalog has optional 'requires' field", func() {
			BeforeEach(func() {
				configFileName = "config_with_requires.yml"
			})

			It("returns config with the requires field", func() {
				Expect(conf.ServiceCatalog.Requires).To(Equal([]string{"syslog_drain", "route_forwarding"}))
				Expect(parseErr).NotTo(HaveOccurred())
			})
		})

		Context("when the BOSH director uses UAA", func() {
			BeforeEach(func() {
				configFileName = "bosh_uaa_config.yml"
			})

			It("returns a config object", func() {
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(conf.Bosh.Authentication.UAA).To(Equal(config.UAAAuthentication{
					ClientCredentials: config.ClientCredentials{
						ID:     "some-client-id",
						Secret: "some-client-secret",
					},
				}))
			})
		})

		Context("when the configuration contains a non-executable service adapter path", func() {
			BeforeEach(func() {
				configFileName = "config_with_non_executable_adapter_path.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError("checking for executable service adapter file: 'test_assets/good_config.yml' is not executable"))
			})
		})

		Context("when the configuration contains an empty service adapter path", func() {
			BeforeEach(func() {
				configFileName = "config_with_missing_adapter_path.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError("checking for executable service adapter file: path is empty"))
			})
		})

		Context("BOSH configuration", func() {
			Context("when the configuration does not specify a BOSH url", func() {
				BeforeEach(func() {
					configFileName = "bosh_no_url_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("BOSH configuration error: must specify bosh url"))
				})
			})

			Context("when the configuration does not specify any BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_no_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(("BOSH configuration error: must specify an authentication type")))
				})
			})

			Context("when the configuration specified incomplete basic BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_bad_basic_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("BOSH configuration error: authentication.basic.password can't be empty"))
				})
			})

			Context("when the configuration specifies incomplete UAA BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_bad_uaa_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("BOSH configuration error: authentication.uaa.client_credentials.client_secret can't be empty"))
				})
			})

			Context("when the configuration specifies both types of BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_both_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("BOSH configuration error: cannot specify both basic and UAA authentication"))
				})
			})
		})

		Context("CF Authentication", func() {
			Context("when the configuration does not specify a CF url", func() {
				BeforeEach(func() {
					configFileName = "cf_no_url_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: must specify CF url"))
				})
			})

			Context("when the CF configuration does not specify any UAA authentication", func() {
				BeforeEach(func() {
					configFileName = "cf_no_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: must specify an authentication type"))
				})
			})

			Context("when the CF configuration does not specify a UAA url", func() {
				BeforeEach(func() {
					configFileName = "cf_no_uaa_url.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: authentication.uaa.url can't be empty"))
				})
			})

			Context("when the CF configuration specified incomplete UAA user credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_bad_user_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: authentication.uaa.user_credentials.password can't be empty"))
				})
			})

			Context("when the CF configuration specifies incomplete UAA client credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_bad_client_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: authentication.uaa.client_credentials.client_secret can't be empty"))
				})
			})

			Context("when the CF configuration specifies both types of UAA credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_both_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: authentication.uaa contains both client and user credentials"))
				})
			})

			Context("when the CF configuration specifies no types of UAA credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_uaa_no_credentials.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError("CF configuration error: authentication.uaa should contain either user_credentials or client_credentials"))
				})
			})
		})

		Context("No CF config", func() {
			BeforeEach(func() {
				configFileName = "no_cf_config.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError("CF configuration error: must specify CF url"))
			})

			Context("and CF checks are disabled", func() {
				BeforeEach(func() {
					configFileName = "cf_checks_disabled_config.yml"
				})

				It("succeeds", func() {
					Expect(parseErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("free flag is not specified", func() {
			BeforeEach(func() {
				configFileName = "config_with_no_free_flag.yml"
			})

			It("returns no error", func() {
				Expect(parseErr).NotTo(HaveOccurred())
			})

			It("parses free as nil", func() {
				Expect(conf.ServiceCatalog.Plans[0].Free).To(BeNil())
			})
		})

		Context("free flag is false", func() {
			BeforeEach(func() {
				configFileName = "config_with_free_false.yml"
			})

			It("returns no error", func() {
				Expect(parseErr).NotTo(HaveOccurred())
			})

			It("parses free as false", func() {
				Expect(*conf.ServiceCatalog.Plans[0].Free).To(BeFalse())
			})
		})

		Context("when the post deploy errand instances property is specified as a/b/c", func() {
			BeforeEach(func() {
				configFileName = "config_with_invalid_post_deploy_instances.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
			})
		})

		Context("when the post deploy errand instances property is specified as /b", func() {
			BeforeEach(func() {
				configFileName = "config_with_invalid_post_deploy_instances_1.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
			})
		})

		Context("when the post deploy errand instances property is specified as a/", func() {
			BeforeEach(func() {
				configFileName = "config_with_invalid_post_deploy_instances_2.yml"
			})

			It("returns an error", func() {
				Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
			})
		})

		Context("pre delete errand", func() {
			Context("when the instances property is specified as a/b/c", func() {
				BeforeEach(func() {
					configFileName = "config_with_invalid_pre_delete_instances.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
				})
			})

			Context("when the instances property is specified as /b", func() {
				BeforeEach(func() {
					configFileName = "config_with_invalid_pre_delete_instances_1.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
				})
			})

			Context("when the instances property is specified as a/", func() {
				BeforeEach(func() {
					configFileName = "config_with_invalid_pre_delete_instances_2.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(MatchRegexp("Must specify pool or instance '.*' in format 'name' or 'name/id-or-index'")))
				})
			})
		})

		Describe("service deployment", func() {
			latestFailureMessage := "You must configure the exact release and stemcell versions in broker.service_deployment. ODB requires exact versions to detect pending changes as part of the 'cf update-service' workflow. For example, latest and 3112.latest are not supported."

			Context("when a release version is latest", func() {
				BeforeEach(func() {
					configFileName = "service_deployment_with_latest_release.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring(latestFailureMessage)))
				})
			})

			Context("when a release version is n.latest", func() {
				BeforeEach(func() {
					configFileName = "service_deployment_with_n_latest_release.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring(latestFailureMessage)))
				})
			})

			Context("when a stemcell version is n.latest", func() {
				BeforeEach(func() {
					configFileName = "service_deployment_with_latest_stemcell.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring(latestFailureMessage)))
				})
			})

			Context("when a release version is invalid", func() {
				BeforeEach(func() {
					configFileName = "service_deployment_with_n_latest_stemcell.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring(latestFailureMessage)))
				})
			})
		})
	})

	Context("when the file does not exist", func() {
		BeforeEach(func() {
			configFileName = "idontexist"
		})

		It("returns an error", func() {
			Expect(os.IsNotExist(parseErr)).To(BeTrue())
		})
	})

	Context("when the file does not contain valid YAML", func() {
		BeforeEach(func() {
			configFileName = "not_yaml.yml"
		})

		It("returns an error", func() {
			Expect(parseErr).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	Describe("serializing to yaml", func() {
		It("doesn't serialize the persistent_disk_type plan field, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							ID:          "optional-disk-plan-id",
							Name:        "optional-disk-plan",
							Description: "optional-disk-plan-description",
							InstanceGroups: []serviceadapter.InstanceGroup{
								{
									VMType: "optional-disk-vm",
								},
							},
						},
					},
				},
			}

			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("persistent_disk_type"))
		})

		It("add arbitrary fields to plan metadata", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							ID:          "optional-disk-plan-id",
							Name:        "optional-disk-plan",
							Description: "optional-disk-plan-description",
							InstanceGroups: []serviceadapter.InstanceGroup{
								{
									VMType: "optional-disk-vm",
								},
							},
							Metadata: config.PlanMetadata{
								AdditionalMetadata: map[string]interface{}{
									"yo": "bill",
								},
							},
						},
					},
				},
			}

			marshalled, err := yaml.Marshal(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(marshalled).To(SatisfyAll(
				Not(ContainSubstring("additional")),
				ContainSubstring("yo: bill"),
			))
		})

		It("add arbitrary fields to service metadata", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Metadata: config.ServiceMetadata{
						AdditionalMetadata: map[string]interface{}{
							"yo": "bill",
						},
					},
					Plans: []config.Plan{
						{
							ID:          "optional-disk-plan-id",
							Name:        "optional-disk-plan",
							Description: "optional-disk-plan-description",
							InstanceGroups: []serviceadapter.InstanceGroup{
								{
									VMType: "optional-disk-vm",
								},
							},
						},
					},
				},
			}

			marshalled, err := yaml.Marshal(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(marshalled).To(SatisfyAll(
				Not(ContainSubstring("additional")),
				ContainSubstring("yo: bill"),
			))
		})

		It("doesn't serialize the metadata/bullets, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							Metadata: config.PlanMetadata{
								Bullets: nil,
							},
						},
					},
				},
			}

			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("bullets"))
		})

		It("doesn't serialize the lifecycle, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							InstanceGroups: []serviceadapter.InstanceGroup{
								{
									Lifecycle: "",
								},
							},
						},
					},
				},
			}
			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("lifecycle:"))
		})

		It("doesn't serialize the update, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							Update: nil,
						},
					},
				},
			}
			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("update"))
		})

		It("doesn't serialize the update/serial, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Plans: []config.Plan{
						{
							Update: &serviceadapter.Update{
								Serial:      nil,
								MaxInFlight: 1,
							},
						},
					},
				},
			}
			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("serial"))
		})

		It("doesn't serialize the requires, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					Requires: nil,
				},
			}

			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("requires"))
		})

		It("doesn't serialize the dashboard_client, if not specified", func() {
			conf := config.Config{
				ServiceCatalog: config.ServiceOffering{
					DashboardClient: nil,
				},
			}

			Expect(yaml.Marshal(conf)).NotTo(ContainSubstring("dashboard_client"))
		})
	})
})

var _ = Describe("ServiceOffering", func() {
	Context("FindPlanByID", func() {
		var offering = config.ServiceOffering{
			Plans: []config.Plan{
				{
					ID:   "planId",
					Name: "planName",
				},
			},
		}

		It("returns the plan if found", func() {
			plan, found := offering.FindPlanByID("planId")

			Expect(found).To(BeTrue())
			Expect(plan.ID).To(Equal("planId"))
			Expect(plan.Name).To(Equal("planName"))
		})

		It("indicates if plan cannot be found", func() {
			_, found := offering.FindPlanByID("not there")
			Expect(found).To(BeFalse())
		})
	})

	Context("HasBindingWithDNSConfigured", func() {
		var conf config.Config
		BeforeEach(func() {
			conf.ServiceCatalog.Plans = []config.Plan{
				{
					ID:   "planId",
					Name: "planName",
					BindingWithDNS: []config.BindingDNS{
						{
							Name:          "foo",
							LinkProvider:  "bar",
							InstanceGroup: "baz",
						},
					},
				},
			}
		})

		It("returns true when 'binding_with_dns' is configured", func() {
			isConfigured := conf.HasBindingWithDNSConfigured()
			Expect(isConfigured).To(BeTrue(), "Expected to return true because a plan is configured with 'binding_with_dns'")
		})

		It("returns false when 'binding_with_dns' is nil", func() {
			conf.ServiceCatalog.Plans[0].BindingWithDNS = nil
			isConfigured := conf.HasBindingWithDNSConfigured()
			Expect(isConfigured).To(BeFalse(), "Expected to return false because the only plan has 'binding_with_dns' configured to be nil")
		})

		It("returns false when 'binding_with_dns' is empty", func() {
			conf.ServiceCatalog.Plans[0].BindingWithDNS = []config.BindingDNS{}
			isConfigured := conf.HasBindingWithDNSConfigured()
			Expect(isConfigured).To(BeFalse(), "Expected to return false because the only plan has 'binding_with_dns' configured to be empty")
		})

	})
})

var _ = Describe("CF#NewAuthHeaderBuilder", func() {
	const tokenToReturn = "auth-token"
	var logger *log.Logger

	BeforeEach(func() {
		logger = log.New(GinkgoWriter, "[config test] ", log.LstdFlags)
	})

	Context("when CF config has user credentials", func() {
		It("returns a user token auth header builder", func() {
			cfConfig := config.CF{
				URL:         "some-cf-url",
				TrustedCert: "some-cf-cert",
				Authentication: config.Authentication{
					UAA: config.UAAAuthentication{
						UserCredentials: config.UserCredentials{
							Username: "some-cf-username",
							Password: "some-cf-password",
						},
					},
				},
			}
			mockUAA := mockuaa.NewUserCredentialsServer(
				"cf",
				"",
				cfConfig.Authentication.UAA.UserCredentials.Username,
				cfConfig.Authentication.UAA.UserCredentials.Password,
				tokenToReturn,
			)
			cfConfig.Authentication.UAA.URL = mockUAA.URL

			builder, err := cfConfig.NewAuthHeaderBuilder(true)
			Expect(err).NotTo(HaveOccurred())

			header := getAuthHeader(builder, logger)
			Expect(header).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

			mockUAA.Close()
		})
	})

	Context("when CF config has client credentials", func() {
		It("returns a client crendentials auth header builder", func() {
			cfConfig := config.CF{
				URL:         "some-cf-url",
				TrustedCert: "some-cf-cert",
				Authentication: config.Authentication{
					UAA: config.UAAAuthentication{
						ClientCredentials: config.ClientCredentials{
							ID:     "some-cf-client-id",
							Secret: "some-cf-client-secret",
						},
					},
				},
			}
			mockUAA := mockuaa.NewClientCredentialsServer(
				cfConfig.Authentication.UAA.ClientCredentials.ID,
				cfConfig.Authentication.UAA.ClientCredentials.Secret,
				tokenToReturn,
			)
			cfConfig.Authentication.UAA.URL = mockUAA.URL

			builder, err := cfConfig.NewAuthHeaderBuilder(true)
			Expect(err).NotTo(HaveOccurred())

			header := getAuthHeader(builder, logger)
			Expect(header).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

			mockUAA.Close()
		})
	})
})

var _ = Describe("Bosh#NewAuthHeaderBuilder", func() {
	var logger *log.Logger

	BeforeEach(func() {
		logger = log.New(GinkgoWriter, "[config test] ", log.LstdFlags)
	})

	It("returns a BasicAuthHeaderBuilder when BOSH config has a basic auth user", func() {
		boshConfig := config.Bosh{
			Authentication: config.Authentication{
				Basic: config.UserCredentials{
					Username: "test-user",
					Password: "super-secret",
				},
			},
		}

		builder, err := boshConfig.NewAuthHeaderBuilder("", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(builder).To(BeAssignableToTypeOf(authorizationheader.BasicAuthHeaderBuilder{}))

		header := getAuthHeader(builder, logger)
		Expect(header).To(ContainSubstring("Basic"))
	})

	It("returns a ClientAuthHeaderBuilder when BOSH config has a UAA property", func() {
		boshConfig := config.Bosh{
			Authentication: config.Authentication{
				UAA: config.UAAAuthentication{
					ClientCredentials: config.ClientCredentials{
						ID:     "test-id",
						Secret: "super-secret",
					},
				},
			},
			TrustedCert: "test-cert",
		}

		tokenToReturn := "test-token"
		mockuaa := mockuaa.NewClientCredentialsServer(
			boshConfig.Authentication.UAA.ClientCredentials.ID,
			boshConfig.Authentication.UAA.ClientCredentials.Secret,
			tokenToReturn,
		)

		builder, err := boshConfig.NewAuthHeaderBuilder(mockuaa.URL, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(builder).To(BeAssignableToTypeOf(&authorizationheader.ClientTokenAuthHeaderBuilder{}))

		header := getAuthHeader(builder, logger)
		Expect(header).To(Equal("Bearer test-token"))
	})

	It("returns an error if no credentials are specified", func() {
		boshConfig := config.Bosh{}

		_, err := boshConfig.NewAuthHeaderBuilder("", true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("No BOSH authentication configured"))
	})
})

var _ = Describe("Config validation", func() {
	DescribeTable("Authentication",
		func(authentication config.Authentication, expectedErr error) {
			err := authentication.Validate(false)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr.Error()))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("succeeds if basic auth is configured", authBlock(validBasicAuthBlock(), config.UAAAuthentication{}), nil),
		Entry(
			"succeeds if UAA auth is configured with user credentials",
			authBlock(config.UserCredentials{}, uaaAuthBlock(validBasicAuthBlock(), config.ClientCredentials{})),
			nil,
		),
		Entry(
			"succeeds if UAA auth is configured with client credentials",
			authBlock(config.UserCredentials{}, uaaAuthBlock(config.UserCredentials{}, validClientCredentialsBlock())),
			nil,
		),
		Entry(
			"fails when neither basic or UAA auth are configured",
			config.Authentication{},
			errors.New("must specify an authentication type"),
		),
		Entry(
			"fails when both basic and UAA auth are configured",
			authBlock(validBasicAuthBlock(), uaaAuthBlock(validBasicAuthBlock(), config.ClientCredentials{})),
			errors.New("cannot specify both basic and UAA authentication"),
		),
		Entry(
			"fails concatenating the fields if basic auth has a field error",
			authBlock(basicAuthBlock("", "password"), config.UAAAuthentication{}),
			errors.New("authentication.basic.username can't be empty"),
		),
		Entry(
			"fails concatenating the fields if UAA client credentials auth has a field error",
			authBlock(config.UserCredentials{}, uaaAuthBlock(config.UserCredentials{}, clientCredsAuthBlock("", "secret"))),
			errors.New("authentication.uaa.client_credentials.client_id can't be empty"),
		),
		Entry(
			"fails describing the error with UAA configuration",
			authBlock(config.UserCredentials{}, uaaAuthBlock(basicAuthBlock("usr", "psw"), clientCredsAuthBlock("id", "secret"))),
			errors.New("authentication.uaa contains both client and user credentials"),
		),
	)

	DescribeTable("UAA Auth",
		func(uaa config.UAAAuthentication, URLRequired bool, expectedErr error) {
			err := uaa.Validate(URLRequired)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr.Error()))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("succeeds if it is correctly configured with user credentials", uaaAuthBlock(validBasicAuthBlock(), config.ClientCredentials{}), false, nil),
		Entry("succeeds if it is correctly configured with client credentials", uaaAuthBlock(config.UserCredentials{}, validClientCredentialsBlock()), false, nil),
		Entry(
			"fails if neither user_credentials or client_credentials are configured and URL is not required",
			uaaAuthBlock(config.UserCredentials{}, config.ClientCredentials{}),
			false,
			errors.New("must specify UAA authentication"),
		),
		Entry(
			"fails if URL is required and specifies user_credentials",
			uaaAuthBlock(validBasicAuthBlock(), config.ClientCredentials{}),
			true,
			errors.New("url can't be empty"),
		),
		Entry(
			"fails if URL is required and specifies client_credentials",
			uaaAuthBlock(config.UserCredentials{}, validClientCredentialsBlock()),
			true,
			errors.New("url can't be empty"),
		),
		Entry(
			"fails if both user_credentials and client_credentials are configured",
			uaaAuthBlock(validBasicAuthBlock(), validClientCredentialsBlock()),
			false,
			errors.New("contains both client and user credentials"),
		),
		Entry(
			"fails if there is a field error in client_credentials",
			uaaAuthBlock(config.UserCredentials{}, clientCredsAuthBlock("", "secret")),
			false,
			errors.New("client_credentials.client_id can't be empty"),
		),
		Entry(
			"fails if there is a field error in user_credentials",
			uaaAuthBlock(basicAuthBlock("", "psw"), config.ClientCredentials{}),
			false,
			errors.New("user_credentials.username can't be empty"),
		),
	)

	DescribeTable("Basic Auth",
		func(basic config.UserCredentials, expectedErr error) {
			err := basic.Validate()
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr.Error()))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("succeeds if it is correctly configured", validBasicAuthBlock(), nil),
		Entry("fails when username is empty", basicAuthBlock("", "psw"), errors.New("username can't be empty")),
		Entry("fails when password is empty", basicAuthBlock("usr", ""), errors.New("password can't be empty")),
	)

	DescribeTable("Client Credentials",
		func(cc config.ClientCredentials, expectedErr error) {
			err := cc.Validate()
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr.Error()))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("succeeds if it is correctly configured", validClientCredentialsBlock(), nil),
		Entry("fails when client_id is empty", clientCredsAuthBlock("", "secret"), errors.New("client_id can't be empty")),
		Entry("fails when client_secret is empty", clientCredsAuthBlock("id", ""), errors.New("client_secret can't be empty")),
	)

})

func authBlock(basic config.UserCredentials, uaa config.UAAAuthentication) config.Authentication {
	return config.Authentication{
		Basic: basic,
		UAA:   uaa,
	}
}

func uaaAuthBlock(userCreds config.UserCredentials, clienCreds config.ClientCredentials) config.UAAAuthentication {
	return config.UAAAuthentication{
		UserCredentials:   userCreds,
		ClientCredentials: clienCreds,
	}
}

func basicAuthBlock(username, password string) config.UserCredentials {
	return config.UserCredentials{
		Username: username,
		Password: password,
	}
}

func clientCredsAuthBlock(id, secret string) config.ClientCredentials {
	return config.ClientCredentials{
		ID:     id,
		Secret: secret,
	}
}

func validBasicAuthBlock() config.UserCredentials {
	return basicAuthBlock("username", "password")
}

func validClientCredentialsBlock() config.ClientCredentials {
	return config.ClientCredentials{
		ID:     "client_id",
		Secret: "client_secret",
	}
}

func booleanPointer(val bool) *bool {
	return &val
}

func getAuthHeader(builder config.AuthHeaderBuilder, logger *log.Logger) string {
	req, err := http.NewRequest("GET", "some-url-to-authorize", nil)
	Expect(err).NotTo(HaveOccurred())
	err = builder.AddAuthHeader(req, logger)
	Expect(err).NotTo(HaveOccurred())
	return req.Header.Get("Authorization")
}
