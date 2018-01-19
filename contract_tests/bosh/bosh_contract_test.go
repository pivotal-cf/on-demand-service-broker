package bosh_test

import (
	"fmt"
	"time"

	"log"

	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("BOSH client", func() {
	var (
		boshClient     *boshdirector.Client
		logger         *log.Logger
		deploymentName string
	)

	BeforeEach(func() {
		SetDefaultEventuallyTimeout(10 * time.Second)
		reporter := boshdirector.NewAsyncTaskReporter()
		boshClient = NewBOSHClient()
		logger = loggerfactory.New(GinkgoWriter, "contract-test", loggerfactory.Flags).New()
		deploymentName = "bill"

		_, err := boshClient.DeleteDeployment(deploymentName, "", logger, reporter)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		reporter := boshdirector.NewAsyncTaskReporter()
		_, err := boshClient.DeleteDeployment(deploymentName, "", logger, reporter)
		Expect(err).NotTo(HaveOccurred())
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
			Expect(uaaURL).To(Equal("https://35.189.248.241:8443"))
		})

		It("is an authenticated director", func() {
			err := boshClient.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteDeployment()", func() {
		It("deletes the deployment and returns a taskID", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("successful_deploy.yml"), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())

			taskID, err := boshClient.DeleteDeployment(deploymentName, "", logger, reporter)
			Expect(taskID).To(BeNumerically(">=", 1))
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())
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

			taskID, err := boshClient.Deploy(getManifest("successful_deploy.yml"), "some-context-id", logger, reporter)

			Expect(err).NotTo(HaveOccurred())
			Expect(taskID).To(BeNumerically(">=", 1))

			Eventually(reporter.Finished).Should(Receive())
		})
	})

	Describe("GetDeployment()", func() {
		It("succeeds and return the manifest when deployment is found", func() {
			manifest := getManifest("successful_deploy.yml")
			reporter := boshdirector.NewAsyncTaskReporter()

			_, err := boshClient.Deploy(manifest, "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(reporter.Finished).Should(Receive())

			returnedManifest, found, getDeploymentErr := boshClient.GetDeployment("bill", logger)

			// TODO: if used somewhere else, extract in suite or Gomega matcher
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
					Alias   string
					OS      string
					version string
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
			_, found, getDeploymentErr := boshClient.GetDeployment("bill", logger)

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
			taskID, err = boshClient.Deploy(getManifest("successful_deploy.yml"), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())
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
			taskID, err = boshClient.Deploy(getManifest("successful_deploy.yml"), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())
		})

		It("succeeds", func() {
			tasks, err := boshClient.GetTasks("bill", logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks[0].ID).To(Equal(taskID))
		})
	})

	Describe("VMs()", func() {
		BeforeEach(func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml"), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(reporter.Finished).Should(Receive())
		})

		It("succeeds", func() {
			vms, err := boshClient.VMs("dummy", logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(vms["dummy"]).NotTo(BeEmpty())
		})
	})

	Describe("VerifyAuth()", func() {
		It("succeeds", func() {
			err := boshClient.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails if the credentials are wrong", func() {
			wrongBoshClient := NewBOSHClientWithBadCredentials()
			err := wrongBoshClient.VerifyAuth(logger)
			Expect(err).To(MatchError(ContainSubstring("Bad credentials")))
		})
	})

	Describe("RunErrand() and GetTaskOutput()", func() {
		It("succeeds", func() {
			reporter := boshdirector.NewAsyncTaskReporter()
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml"), "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(reporter.Finished).Should(Receive())

			By("running the errand")
			reporter = boshdirector.NewAsyncTaskReporter()
			taskId, err := boshClient.RunErrand("dummy", "dummy_errand", nil, "", logger, reporter)
			Expect(err).NotTo(HaveOccurred())

			By("Getting the task output")
			output, err := boshClient.GetTaskOutput(taskId, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.StdOut).To(Equal("running dummy errand\n"))
		})
	})
})

func getManifest(filename string) []byte {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("../fixtures/%s", filename))
	Expect(err).NotTo(HaveOccurred())
	return bytes
}
