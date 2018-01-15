package bosh_test

import (
	"fmt"

	"log"

	"io/ioutil"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"gopkg.in/yaml.v2"
)

var _ = Describe("BOSH client", func() {
	var (
		director       director.Director
		boshClient     *boshdirector.Client
		logger         *log.Logger
		deploymentName string
	)

	BeforeEach(func() {
		director = getDirector()
		boshClient = boshdirector.NewBOSHClient(director)
		logger = loggerfactory.New(GinkgoWriter, "contract-test", loggerfactory.Flags).New()
		deploymentName = "bill"

		_, err := boshClient.DeleteDeployment(deploymentName, "", logger)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, err := boshClient.DeleteDeployment(deploymentName, "", logger)
		Expect(err).NotTo(HaveOccurred())
	})

	// TODO: this should be our wrapper
	XDescribe("Info()", func() {
		It("talks to the director", func() {
			info, err := director.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Name).To(Equal("bosh-lite-director"))
		})

		It("gets the info about director", func() {
			info, err := director.Info()
			fmt.Println(info)
			Expect(err).NotTo(HaveOccurred())

			uaaURL, ok := info.Auth.Options["url"].(string)
			Expect(ok).To(BeTrue(), "Cannot retrieve UAA url from /info")

			Expect(uaaURL).To(Equal("https://35.189.248.241:8443"))
		})

		It("is an authenticated director", func() {
			isAuth, err := director.IsAuthenticated()
			Expect(isAuth).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteDeployment()", func() {
		It("delete the deployment and returns a taskID", func() {
			_, err := boshClient.Deploy(getManifest("successful_deploy.yml"), "", logger)
			Expect(err).NotTo(HaveOccurred())

			taskID, err := boshClient.DeleteDeployment(deploymentName, "", logger)
			Expect(taskID).To(BeNumerically(">=", 1))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 0 for task ID and no error when a deployment does not exist", func() {
			taskID, err := boshClient.DeleteDeployment("something-that-does-not-exist", "", logger)
			Expect(taskID).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Deploy()", func() {
		It("succeeds", func() {
			taskID, err := boshClient.Deploy(getManifest("successful_deploy.yml"), "some-context-id", logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(taskID).To(BeNumerically(">=", 1))
		})
	})

	Describe("GetDeployment()", func() {
		It("succeeds and return the manifest when deployment is found", func() {
			manifest := getManifest("successful_deploy.yml")
			_, err := boshClient.Deploy(manifest, "", logger)
			Expect(err).NotTo(HaveOccurred())

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
			_, err := boshClient.DeleteDeployment("bill", "some-context-id", logger)
			Expect(err).NotTo(HaveOccurred())

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

	Describe("GetTask()", func() {})

	Describe("GetTaskOutput()", func() {
		It("returns the task output", func() {
			_, err := boshClient.Deploy(getManifest("single_vm_deployment.yml"), "", logger)
			Expect(err).NotTo(HaveOccurred())

			taskId, err := boshClient.RunErrand("dummy", "dummy_errand", nil, "", logger)
			Expect(err).NotTo(HaveOccurred())

			output, err := boshClient.GetTaskOutput(taskId, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(output[0].StdOut).To(Equal("running dummy errand\n"))
		})
	})
})

func getManifest(filename string) []byte {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("../fixtures/%s", filename))
	Expect(err).NotTo(HaveOccurred())
	return bytes
}
