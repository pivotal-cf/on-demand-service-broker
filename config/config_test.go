// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package config_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Config", func() {
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
						Port:                        8080,
						Username:                    "username",
						Password:                    "password",
						DisableSSLCertVerification:  true,
						StartUpBanner:               false,
						GracefulHTTPShutdownTimeout: 10,
					},
					Bosh: config.Bosh{
						URL:         "some-url",
						TrustedCert: "some-cert",
						Authentication: config.BOSHAuthentication{
							Basic: config.UserCredentials{
								Username: "some-username",
								Password: "some-password",
							},
						},
					},
					CF: config.CF{
						URL:         "some-cf-url",
						TrustedCert: "some-cf-cert",
						Authentication: config.UAAAuthentication{
							URL: "a-uaa-url",
							UserCredentials: config.UserCredentials{
								Username: "some-cf-username",
								Password: "some-cf-password",
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
								LifecycleErrands: &config.LifecycleErrands{
									PostDeploy: "health-check",
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
										Instances:    1,
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

				Expect(conf).To(Equal(expected))
			})

			It("returns no error", func() {
				Expect(parseErr).NotTo(HaveOccurred())
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
				Expect(conf.Bosh.Authentication.UAA).To(Equal(config.BOSHUAAAuthentication{
					UAAURL: "http://some-uaa-server:99",
					ID:     "some-client-id",
					Secret: "some-client-secret",
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
					Expect(parseErr).To(MatchError(ContainSubstring("Must specify bosh url")))
				})
			})

			Context("when the configuration does not specify any BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_no_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Must specify bosh authentication")))
				})
			})

			Context("when the configuration specified incomplete basic BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_bad_basic_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("basic.password can't be empty")))
				})
			})

			Context("when the configuration specifies incomplete UAA BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_bad_uaa_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("uaa.secret can't be empty")))
				})
			})

			Context("when the configuration specifies both types of BOSH authentication", func() {
				BeforeEach(func() {
					configFileName = "bosh_both_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Cannot specify both basic and UAA for BOSH authentication")))
				})
			})
		})

		Context("CF Authentication", func() {
			Context("when the configuration does not specify a CF url", func() {
				BeforeEach(func() {
					configFileName = "cf_no_url_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Must specify CF url")))
				})
			})

			Context("when the CF configuration does not specify any UAA authentication", func() {
				BeforeEach(func() {
					configFileName = "cf_no_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Must specify UAA authentication")))
				})
			})

			Context("when the CF configuration does not specify a UAA url", func() {
				BeforeEach(func() {
					configFileName = "cf_no_uaa_url.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Must specify UAA url")))
				})
			})

			Context("when the CF configuration specified incomplete UAA user credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_bad_user_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("user_credentials.password can't be empty")))
				})
			})

			Context("when the CF configuration specifies incomplete UAA client credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_bad_client_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("client_credentials.secret can't be empty")))
				})
			})

			Context("when the CF configuration specifies both types of UAA credentials", func() {
				BeforeEach(func() {
					configFileName = "cf_both_auth_config.yml"
				})

				It("returns an error", func() {
					Expect(parseErr).To(MatchError(ContainSubstring("Cannot specify both client and user credentials for UAA authentication")))
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
								Serial: nil,
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
				Authentication: config.UAAAuthentication{
					UserCredentials: config.UserCredentials{
						Username: "some-cf-username",
						Password: "some-cf-password",
					},
				},
			}
			mockUAA := mockuaa.NewUserCredentialsServer(
				"cf",
				"",
				cfConfig.Authentication.UserCredentials.Username,
				cfConfig.Authentication.UserCredentials.Password,
				tokenToReturn,
			)
			cfConfig.Authentication.URL = mockUAA.URL

			builder, err := cfConfig.NewAuthHeaderBuilder(true)
			Expect(err).NotTo(HaveOccurred())

			header, err := builder.Build(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(header).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

			mockUAA.Close()
		})
	})

	Context("when CF config has client credentials", func() {
		It("returns a client crendentials auth header builder", func() {
			cfConfig := config.CF{
				URL:         "some-cf-url",
				TrustedCert: "some-cf-cert",
				Authentication: config.UAAAuthentication{
					ClientCredentials: config.ClientCredentials{
						ID:     "some-cf-client-id",
						Secret: "some-cf-client-secret",
					},
				},
			}
			mockUAA := mockuaa.NewClientCredentialsServer(
				cfConfig.Authentication.ClientCredentials.ID,
				cfConfig.Authentication.ClientCredentials.Secret,
				tokenToReturn,
			)
			cfConfig.Authentication.URL = mockUAA.URL

			builder, err := cfConfig.NewAuthHeaderBuilder(true)
			Expect(err).NotTo(HaveOccurred())

			header, err := builder.Build(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(header).To(Equal(fmt.Sprintf("Bearer %s", tokenToReturn)))

			mockUAA.Close()
		})
	})
})

func booleanPointer(val bool) *bool {
	return &val
}
