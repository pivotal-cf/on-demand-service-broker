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

package bosh_test

import (
	"fmt"
	"time"

	"log"

	"io/ioutil"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"gopkg.in/yaml.v2"
)

var _ = Describe("BOSH client", func() {
	var (
		boshClient     *boshdirector.Client
		logger         *log.Logger
		deploymentName string
	)

	BeforeEach(func() {
		SetDefaultEventuallyTimeout(1 * time.Minute)
		boshClient = NewBOSHClient()
		logger = loggerfactory.New(GinkgoWriter, "contract-test", loggerfactory.Flags).New()
		deploymentName = "bill-" + uuid.New()
	})

	AfterEach(func() {
		reporter := boshdirector.NewAsyncTaskReporter()
		_, err := boshClient.DeleteDeployment(deploymentName, "", logger, reporter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for deployment %s to be deleted", deploymentName))
	})

	verifyContextID := func(expectedContextID string, taskID int) {
		task, err := boshClient.GetTask(taskID, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(task.ContextID).To(Equal(expectedContextID))
	}

	Describe("GetInfo()", func() {
		It("talks to the director", func() {
			info, err := boshClient.GetInfo(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Version).NotTo(BeEmpty())
		})

		It("gets the info about director", func() {
			info, err := boshClient.GetInfo(logger)

			Expect(err).NotTo(HaveOccurred())
			uaaURL := info.UserAuthentication.Options.URL
			Expect(uaaURL).To(Equal(boshAuthConfig.UAA.URL))
		})

		It("is an authenticated director", func() {
			err := boshClient.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteDeployment()", func() {
		It("deletes the deployment and returns a taskID", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("successful_deploy.yml", deploymentName), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))

			reporter = boshdirector.NewAsyncTaskReporter()
			taskID, err := boshClient.DeleteDeployment(deploymentName, "some-context-id", logger, reporter)
			Expect(taskID).To(BeNumerically(">=", 1))
			Expect(err).NotTo(HaveOccurred())
			verifyContextID("some-context-id", taskID)
			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for deployment %s to be deleted", deploymentName))
		})

		It("returns 0 for task ID and no error when a deployment does not exist", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			taskID, err := boshClient.DeleteDeployment("something-that-does-not-exist", "", logger, reporter)
			Expect(taskID).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())
		})
	})

	Describe("Deploy()", func() {
		It("succeeds", func() {
			reporter := boshdirector.NewAsyncTaskReporter()

			taskID, err := boshClient.Deploy(getManifest("successful_deploy.yml", deploymentName), "some-context-id", logger, reporter)

			Expect(err).NotTo(HaveOccurred())
			Expect(taskID).To(BeNumerically(">=", 1))

			verifyContextID("some-context-id", taskID)
			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))
		})
	})

	Describe("GetDeployment()", func() {
		It("succeeds and return the manifest when deployment is found", func() {
			manifest := getManifest("successful_deploy.yml", deploymentName)
			reporter := boshdirector.NewAsyncTaskReporter()

			_, err := boshClient.Deploy(manifest, "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))

			returnedManifest, found, getDeploymentErr := boshClient.GetDeployment(deploymentName, logger)

			type testManifest struct {
				Name     string
				Releases []string
				Update   struct {
					Canaries        int
					CanaryWatchTime string `yaml:"canary_watch_time"`
					UpdateWatchTime string `yaml:"update_watch_time"`
					MaxInFlight     int    `yaml:"max_in_flight"`
				}
				StemCells []struct {
					Alias string
					OS    string
				}
			}

			var marshalledManifest testManifest
			var marshalledReturnedManifest testManifest
			err = yaml.Unmarshal(manifest, &marshalledManifest)
			Expect(err).NotTo(HaveOccurred())
			err = yaml.Unmarshal(returnedManifest, &marshalledReturnedManifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(marshalledManifest).To(Equal(marshalledReturnedManifest))
			Expect(found).To(BeTrue())
			Expect(getDeploymentErr).NotTo(HaveOccurred())
		})

		It("does not fail when deployment is not found", func() {
			_, found, getDeploymentErr := boshClient.GetDeployment("sad-face", logger)

			Expect(found).To(BeFalse())
			Expect(getDeploymentErr).NotTo(HaveOccurred())
		})
	})

	Describe("GetDeployments()", func() {
		It("succeeds", func() {
			deployments, err := boshClient.GetDeployments(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployments).To(ContainElement(boshdirector.Deployment{Name: "cf"}))
		})
	})

	Describe("GetTask()", func() {
		var taskID int
		var err error

		BeforeEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			taskID, err = boshClient.Deploy(getManifest("successful_deploy.yml", deploymentName), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))
		})

		It("succeeds", func() {
			task, err := boshClient.GetTask(taskID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.ID).To(Equal(taskID))
		})
	})

	Describe("GetTasks()", func() {
		var taskID int
		var err error

		BeforeEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			taskID, err = boshClient.Deploy(getManifest("successful_deploy.yml", deploymentName), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))
		})

		It("succeeds", func() {
			tasks, err := boshClient.GetTasks(deploymentName, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks[0].ID).To(Equal(taskID))
		})
	})

	Describe("VMs()", func() {
		BeforeEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml", deploymentName), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))
		})

		It("succeeds", func() {
			vms, err := boshClient.VMs(deploymentName, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(vms["dummy"]).NotTo(BeEmpty())
		})
	})

	Describe("VerifyAuth()", func() {
		It("validates the credentials", func() {
			By("succeeding when the creds are correct")
			err := boshClient.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())

			By("failing when the creds are wrong")
			wrongBoshClient := NewBOSHClientWithBadCredentials()
			err = wrongBoshClient.VerifyAuth(logger)
			Expect(err).To(MatchError(ContainSubstring("Bad credentials")))
		})
	})

	Describe("RunErrand() and GetTaskOutput()", func() {
		It("succeeds", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml", deploymentName), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))

			By("running the errand")
			reporter = boshdirector.NewAsyncTaskReporter()
			taskId, err := boshClient.RunErrand(deploymentName, "dummy_errand", nil, "some-context-id", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			By("executing with the correct context id")
			verifyContextID("some-context-id", taskId)

			By("Getting the task output")
			output, err := boshClient.GetTaskOutput(taskId, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.StdOut).To(Equal("running dummy errand\n"))
		})
	})

	Describe("Variables()", func() {
		It("returns the variables for the deployment", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml", deploymentName), "some-context-id", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))

			variables, err := boshClient.Variables(deploymentName, logger)
			Expect(err).NotTo(HaveOccurred())

			By("Not returning the bosh variables for direct credhub references without a corresponding entry in bosh variables section")
			Expect(variables).To(HaveLen(2))
			Expect(variables[0].Path).To(ContainSubstring("a-var"))
			Expect(variables[1].Path).To(MatchRegexp("my-cert$"))
		})
	})

	Describe("raw httpclient commands", func() {
		var (
			cloudConfigJSON = `{"name": "test", "type": "cloud", "content": "--- {}"}`
			boshHTTP        boshdirector.HTTP
		)

		BeforeEach(func() {
			boshHTTP = boshdirector.NewBoshHTTP(boshClient)
		})

		Describe("RawGet", func() {
			It("returns valid info", func() {
				r, err := boshHTTP.RawGet("/info")
				Expect(err).NotTo(HaveOccurred())

				boshInfo := struct {
					Name string
					UUID string
					User string
					CPI  string
				}{}

				err = json.Unmarshal([]byte(r), &boshInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(boshInfo.Name).ToNot(BeEmpty())
				Expect(boshInfo.UUID).ToNot(BeEmpty())
				Expect(boshInfo.User).ToNot(BeEmpty())
				Expect(boshInfo.CPI).ToNot(BeEmpty())
			})
		})

		Describe("RawPost()", func() {
			AfterEach(func() {
				_, err := boshHTTP.RawDelete("/configs?type=cloud&name=test")
				Expect(err).NotTo(HaveOccurred())
			})

			It("can set cloud config", func() {
				r, err := boshHTTP.RawPost("/configs", cloudConfigJSON, "application/json")
				Expect(err).NotTo(HaveOccurred())

				postResponse := struct {
					Content string
					Name    string
					Type    string
				}{}

				err = json.Unmarshal([]byte(r), &postResponse)
				Expect(err).NotTo(HaveOccurred())
				Expect(postResponse.Content).To(Equal("--- {}"))
				Expect(postResponse.Name).To(Equal("test"))
				Expect(postResponse.Type).To(Equal("cloud"))
			})
		})

		Describe("RawDelete()", func() {
			BeforeEach(func() {
				_, err := boshHTTP.RawPost("/configs", cloudConfigJSON, "application/json")
				Expect(err).NotTo(HaveOccurred())
			})

			It("can delete a cloud config", func() {
				_, err := boshHTTP.RawDelete("/configs?type=cloud&name=test")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("LinksAPI", func() {
		var dnsRetriever boshdirector.DNSRetriever
		BeforeEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("deployment_with_link_provider.yml", deploymentName), "some-context-id", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))

			dnsRetriever = boshlinks.NewDNSRetriever(boshdirector.NewBoshHTTP(boshClient))
		})

		AfterEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.DeleteDeployment(deploymentName, "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uses bosh links api calls to get bosh dns address", func() {
			instanceGroupName := "dummy"
			providerName := "link_from_dummy"

			linkProviderId, err := dnsRetriever.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).NotTo(HaveOccurred())

			linkConsumerId, err := dnsRetriever.CreateLinkConsumer(linkProviderId)
			Expect(err).NotTo(HaveOccurred())

			linkAddress, err := dnsRetriever.GetLinkAddress(linkConsumerId, nil, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(linkAddress).To(MatchRegexp(`\.dummy\..*\.bosh$`))

			linkAddressWithAZs, err := dnsRetriever.GetLinkAddress(linkConsumerId, []string{"z1", "z2"}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(linkAddressWithAZs).To(MatchRegexp(`\.dummy\..*\.bosh$`))
			Expect(linkAddressWithAZs).NotTo(Equal(linkAddress))

			linkAddressWithStatus, err := dnsRetriever.GetLinkAddress(linkConsumerId, []string{}, "healthy")
			Expect(err).NotTo(HaveOccurred())
			Expect(linkAddressWithStatus).To(MatchRegexp(`\.dummy\..*\.bosh$`))
			Expect(linkAddressWithStatus).NotTo(Equal(linkAddress))
			Expect(linkAddressWithStatus).NotTo(Equal(linkAddressWithAZs))
			err = dnsRetriever.DeleteLinkConsumer(linkConsumerId)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func getManifest(filename, deploymentName string) []byte {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("../fixtures/%s", filename))
	Expect(err).NotTo(HaveOccurred())
	return append(bytes, []byte("\nname: "+deploymentName)...)
}
