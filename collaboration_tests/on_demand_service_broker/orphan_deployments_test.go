// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package on_demand_service_broker_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("Orphan Deployments", func() {
	var orphanDeploymentsBinary, errandConfigPath string

	BeforeEach(func() {
		var err error
		orphanDeploymentsBinary, err = gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/orphan-deployments")
		Expect(err).ToNot(HaveOccurred())

		errandConfigPath = write(brokerConfig.OrphanDeploymentsErrandConfig{
			BrokerAPI: brokerConfig.BrokerAPI{
				URL: "http://" + serverURL,
				Authentication: brokerConfig.Authentication{
					Basic: brokerConfig.UserCredentials{
						Username: brokerUsername,
						Password: brokerPassword,
					},
				},
			},
		})
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
		os.Remove(errandConfigPath)
	})

	Context("with CF configured in the broker", func() {
		JustBeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
					Plans: brokerConfig.Plans{
						{Name: dedicatedPlanName, ID: dedicatedPlanID},
						{Name: highMemoryPlanName, ID: highMemoryPlanID},
					},
				},
			}

			StartServer(conf)
		})

		It("exits 0 when no orphan deployments are found", func() {
			cmd := exec.Command(orphanDeploymentsBinary, "-configPath", errandConfigPath)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(1), "CF Client wasn't called")
			Expect(session).To(gbytes.Say(`\[\]`))
		})

		Context("with a single bosh deployment", func() {
			BeforeEach(func() {
				fakeBoshClient.GetDeploymentsReturns([]boshdirector.Deployment{
					{Name: "service-instance_1"},
				}, nil)
			})

			It("exits 10 when orphan deployments are found through CF", func() {
				cmd := exec.Command(orphanDeploymentsBinary, "-configPath", errandConfigPath)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(10), "expected exit code 10 for presence of orphans")

				Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(1), "CF Client wasn't called")
				Expect(session).To(gbytes.Say(`\[{"deployment_name":"service-instance_1"}\]`))
			})
		})
	})

	Context("with SI API configured in the broker", func() {
		var SIAPIServer *ghttp.Server

		JustBeforeEach(func() {
			SIAPIServer = ghttp.NewServer()

			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port:     serverPort,
					Username: brokerUsername,
					Password: brokerPassword,
				},
				ServiceInstancesAPI: brokerConfig.ServiceInstancesAPI{
					URL: SIAPIServer.URL(),
					Authentication: brokerConfig.Authentication{
						Basic: brokerConfig.UserCredentials{
							Username: "foo",
							Password: "bar",
						},
					},
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
					Plans: brokerConfig.Plans{
						{Name: dedicatedPlanName, ID: dedicatedPlanID},
						{Name: highMemoryPlanName, ID: highMemoryPlanID},
					},
				},
			}

			StartServer(conf)
		})

		AfterEach(func() {
			SIAPIServer.Close()
		})

		Context("with 1 deployment in BOSH", func() {
			BeforeEach(func() {
				fakeBoshClient.GetDeploymentsReturns([]boshdirector.Deployment{
					{Name: "service-instance_1"},
				}, nil)
			})

			It("exits 10 when orphan deployments found through SI API", func() {
				SIAPIServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/"),
						ghttp.VerifyBasicAuth("foo", "bar"),
						ghttp.RespondWith(http.StatusOK, `[]`, http.Header{"Content-type": {"application/json"}}),
					),
				)

				cmd := exec.Command(orphanDeploymentsBinary, "-configPath", errandConfigPath)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				Eventually(session).Should(gexec.Exit(10), "expected exit code 10 for presence of orphans")
				Expect(SIAPIServer.ReceivedRequests()).To(HaveLen(1), "No request was sent through the SI API")

				Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(0))
				Expect(session).To(gbytes.Say(`\[{"deployment_name":"service-instance_1"}\]`))
			})

			It("exits 0 when there are no orphans", func() {
				SIAPIServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"),
					ghttp.VerifyBasicAuth("foo", "bar"),
					ghttp.RespondWith(http.StatusOK, `[{"service_instance_id":"1"}]`, http.Header{"Content-type": {"application/json"}}),
				))

				cmd := exec.Command(orphanDeploymentsBinary, "-configPath", errandConfigPath)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0), "expected exit code 0 for presence of orphans")

				Expect(session).To(gbytes.Say(`\[\]`))

				Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(0), "Unexpected call to CF")
			})
		})
	})
})

func write(c interface{}) string {
	b, err := yaml.Marshal(c)
	Expect(err).ToNot(HaveOccurred(), "can't marshal orphan deployment errand config")

	file, err := ioutil.TempFile("", "config")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	_, err = file.Write(b)
	Expect(err).NotTo(HaveOccurred())

	return file.Name()
}
