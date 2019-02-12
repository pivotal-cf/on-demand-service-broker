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
	"regexp"
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
)

var _ = Describe("BOSH client", func() {
	var (
		boshClient     *boshdirector.Client
		logger         *log.Logger
		deploymentName string
	)

	BeforeEach(func() {
		SetDefaultEventuallyTimeout(30 * time.Minute)
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

	Describe("deployment operations", func() {
		It("can control the entire lifecycle of a deployment", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			manifest := getManifest("single_vm_deployment.yml", deploymentName)
			var (
				task         boshdirector.BoshTask
				taskID       int
				errandTaskID int
			)

			By("deploying the manifest", func() {
				var err error
				taskID, err = boshClient.Deploy(manifest, "some-context-id", logger, reporter)
				Expect(err).NotTo(HaveOccurred())
				Expect(taskID).To(BeNumerically(">=", 1))
				verifyContextID("some-context-id", taskID)
				Eventually(reporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for %s to deploy", deploymentName))
				Expect(reporter.State).ToNot(Equal("error"), fmt.Sprintf("Deployment of %s failed", deploymentName))
			})

			By("checking all the deployed vms", func() {
				deployments, getDeploymentErr := boshClient.GetDeployments(logger)
				Expect(getDeploymentErr).NotTo(HaveOccurred())
				Expect(deployments).To(ContainElement(boshdirector.Deployment{Name: deploymentName}))
			})

			By("checking that the deployment is there", func() {
				returnedManifest, found, getDeploymentErr := boshClient.GetDeployment(deploymentName, logger)
				Expect(found).To(BeTrue())
				Expect(getDeploymentErr).NotTo(HaveOccurred())
				Expect(returnedManifest).To(MatchYAML(manifest))
			})

			By("verifying a single task", func() {
				var err error
				task, err = boshClient.GetTask(taskID, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.ID).To(Equal(taskID))
			})

			By("verifying all tasks", func() {
				tasks, err := boshClient.GetTasks(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(ContainElement(task))
			})

			By("pulling VM information", func() {
				vms, err := boshClient.VMs(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(vms["dummy"]).NotTo(BeEmpty())
			})

			By("pulling the bosh variables", func() {
				variables, err := boshClient.Variables(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())

				odbVar1Matcher, err := regexp.Compile("a-var$")
				Expect(err).NotTo(HaveOccurred())
				odbVar2Matcher, err := regexp.Compile("my-cert$")
				Expect(err).NotTo(HaveOccurred())

				var var1Found bool
				var var2Found bool

				for _, variable := range variables {
					if odbVar1Matcher.Match([]byte(variable.Path)) {
						var1Found = true
					} else if odbVar2Matcher.Match([]byte(variable.Path)) {
						var2Found = true
					}
				}
				Expect(var1Found).To(BeTrue())
				Expect(var2Found).To(BeTrue())
			})

			By("running an errand", func() {
				var err error
				errandReporter := boshdirector.NewAsyncTaskReporter()
				errandTaskID, err = boshClient.RunErrand(deploymentName, "dummy_errand", nil, "some-context-id", logger, errandReporter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(errandReporter.Finished).Should(Receive(), "timed out waiting for errand")
				verifyContextID("some-context-id", errandTaskID)
			})

			By("getting the task output", func() {
				output, err := boshClient.GetTaskOutput(errandTaskID, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(output.StdOut).To(Equal("running dummy errand\n"))
			})

			By("recreating the deployment", func() {
				recreateCtx := "recreate-context-1"
				recreateReporter := boshdirector.NewAsyncTaskReporter()
				recreateTaskID, err := boshClient.Recreate(deploymentName, recreateCtx, logger, recreateReporter)
				Expect(err).NotTo(HaveOccurred())
				Expect(recreateTaskID).To(BeNumerically(">", 0))
				// verifyContextID() should be uncommented when BOSH has merged and released https://github.com/cloudfoundry/bosh/pull/2084
				// verifyContextID(recreateCtx, recreateTaskID)
				Eventually(recreateReporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for deployment %s to be recreated", deploymentName))
				Expect(reporter.State).ToNot(Equal("error"), fmt.Sprintf("Recreation of %s failed", deploymentName))
			})

			By("deleting the deployment", func() {
				deleteDeploymentReporter := boshdirector.NewAsyncTaskReporter()
				deleteTaskID, err := boshClient.DeleteDeployment(deploymentName, "some-context-id", logger, deleteDeploymentReporter)
				Expect(deleteTaskID).To(BeNumerically(">=", 1))
				Expect(err).NotTo(HaveOccurred())
				verifyContextID("some-context-id", deleteTaskID)
				Eventually(deleteDeploymentReporter.Finished).Should(Receive(), fmt.Sprintf("Timed out waiting for deployment %s to be deleted", deploymentName))

				By("verifying the deployment was deleted")
				deployments, getDeploymentErr := boshClient.GetDeployments(logger)
				Expect(getDeploymentErr).NotTo(HaveOccurred())
				Expect(deployments).NotTo(ContainElement(boshdirector.Deployment{Name: deploymentName}))
			})
		})
	})

	Describe("when a deployment doesn't exist", func() {
		When("DeleteDeployment is called", func() {
			It("returns 0 for task ID and no error", func() {
				reporter := boshdirector.NewAsyncTaskReporter()
				taskID, err := boshClient.DeleteDeployment("something-that-does-not-exist", "", logger, reporter)
				Expect(taskID).To(Equal(0))
				Expect(err).NotTo(HaveOccurred())

				Eventually(reporter.Finished).Should(Receive())
			})
		})
		When("GetDeployment is called", func() {
			It("returns false and no error", func() {
				_, found, getDeploymentErr := boshClient.GetDeployment("sad-face", logger)
				Expect(found).To(BeFalse())
				Expect(getDeploymentErr).NotTo(HaveOccurred())
			})
		})
	})

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
			Expect(reporter.State).ToNot(Equal("error"), "Failed to deploy dummy deployment")

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

	Describe("configs operations", func() {
		var (
			configType           = "dummy"
			anotherConfigType    = "froop"
			configContent        = []byte("vm_extensions: [{name: dummy}]")
			anotherConfigContent = []byte("vm_extensions: [{name: froop}]")
		)

		AfterEach(func() {
			_, err := boshClient.DeleteConfig(configType, deploymentName, logger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can control the entire lifecycle of a config", func() {
			By("updating a config", func() {
				err := boshClient.UpdateConfig(configType, deploymentName, configContent, logger)
				Expect(err).NotTo(HaveOccurred())
			})

			By("checking all the configs", func() {
				configs, err := boshClient.GetConfigs(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(configs)).To(Equal(1))
				Expect(configs).To(ContainElement(boshdirector.BoshConfig{Type: configType, Name: deploymentName, Content: string(configContent)}))
			})

			By("adding another config with the same name but different type", func() {
				err := boshClient.UpdateConfig(anotherConfigType, deploymentName, anotherConfigContent, logger)
				Expect(err).NotTo(HaveOccurred())
			})

			By("checking list configs now returns both configs", func() {
				configs, err := boshClient.GetConfigs(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(configs)).To(Equal(2))
			})

			By("deleting configs", func() {
				found, err := boshClient.DeleteConfig(configType, deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = boshClient.DeleteConfig(anotherConfigType, deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				By("verifying the configs were deleted")
				configs, err := boshClient.GetConfigs(deploymentName, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(configs)).To(Equal(0))
			})
		})
	})
})

func getManifest(filename, deploymentName string) []byte {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("../fixtures/%s", filename))
	Expect(err).NotTo(HaveOccurred())
	return append(bytes, []byte("\nname: "+deploymentName)...)
}
