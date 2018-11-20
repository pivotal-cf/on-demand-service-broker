package recreate_all_service_instances_test

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os/exec"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/collaboration_tests/helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"

	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Recreate all service instances", func() {
	const (
		brokerUsername    = "some-user"
		brokerPassword    = "some-password"
		serviceName       = "service-name"
		dedicatedPlanID   = "dedicated-plan-id"
		dedicatedPlanName = "dedicated-plan-name"
	)

	var (
		serverPort = rand.Intn(math.MaxInt16-1024) + 1024
		serverURL  = fmt.Sprintf("http://localhost:%d", serverPort)

		brokerServer *helpers.Server
	)

	Describe("A successful recreate-all", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
					Plans: brokerConfig.Plans{
						{
							Name: dedicatedPlanName,
							ID:   dedicatedPlanID,
							LifecycleErrands: &serviceadapter.LifecycleErrands{
								PostDeploy: []serviceadapter.Errand{
									serviceadapter.Errand{
										Name: "init-cluster",
									},
								},
							},
						},
					},
				},
			}
			brokerServer = StartServer(conf)
		})

		AfterEach(func() {
			brokerServer.Close()
		})

		It("recreates all service instances", func() {
			errandConfig := brokerConfig.InstanceIteratorConfig{
				PollingInterval: 1,
				AttemptInterval: 1,
				AttemptLimit:    1,
				RequestTimeout:  1,
				MaxInFlight:     1,
				BrokerAPI: brokerConfig.BrokerAPI{
					URL: serverURL,
					Authentication: brokerConfig.Authentication{
						Basic: brokerConfig.UserCredentials{
							Username: brokerUsername,
							Password: brokerPassword,
						},
					},
				},
				ServiceInstancesAPI: brokerConfig.ServiceInstancesAPI{
					URL: serverURL + "/mgmt/service_instances",
					Authentication: brokerConfig.Authentication{
						Basic: brokerConfig.UserCredentials{
							Username: brokerUsername,
							Password: brokerPassword,
						},
					},
				},
			}
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{
				{GUID: "service-1", PlanUniqueID: dedicatedPlanID},
				{GUID: "service-2", PlanUniqueID: dedicatedPlanID},
			}, nil)

			fakeTaskBoshClient.RecreateReturns(42, nil)
			fakeBoshClient.GetTaskReturns(boshdirector.BoshTask{
				ID:          42,
				State:       boshdirector.TaskDone,
				Description: "",
				Result:      "",
				ContextID:   "",
			}, nil)

			/*
			* Test case:
			*		- setup instances to be two
			*		- setup the broker
			*		  - Requests:
			*				* list all instances /mgmt/instances
			*				* recreate first      /mgmt/recreate/guid PUT
			*				*	wait it complete	 last_operation?
			*				* recreate second     /mgmt/recreate/guid PUT
			*				*	wait it complete	 last_operation?
			*		- Run the binary
			*		- Assert on exit code
			*		- Assert on logging (?)
			* */

			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			cmd := exec.Command(pathToRecreateAll, "--configPath", toFilePath(errandConfig))
			session, err := gexec.Start(cmd, stdout, stderr)
			Expect(err).NotTo(HaveOccurred(), "unexpected error when starting the command")

			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).To(Equal(0), "recreate-all execution failed")

			expectedRecreateCallCount := 2
			Expect(fakeTaskBoshClient.RecreateCallCount()).To(Equal(expectedRecreateCallCount), "Recreate() wasn't called twice")
			instancesRecreated := []string{}
			for i := 0; i < expectedRecreateCallCount; i++ {
				deploymentName, _, _, _ := fakeTaskBoshClient.RecreateArgsForCall(i)
				instancesRecreated = append(instancesRecreated, deploymentName)
			}
			Expect(instancesRecreated).To(ConsistOf("service-instance_service-1", "service-instance_service-2"))

			Expect(stdout).To(gbytes.Say("Starting to process service instance 1 of 2"))
			Expect(stdout).To(gbytes.Say("Starting to process service instance 2 of 2"))
			Expect(stdout).To(gbytes.Say(`\[recreate-all\] FINISHED PROCESSING Status: SUCCESS`))

			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(0))
		})

	})
})

func toFilePath(c brokerConfig.InstanceIteratorConfig) string {
	file, err := ioutil.TempFile("", "config")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	b, err := yaml.Marshal(c)
	Expect(err).NotTo(HaveOccurred(), "failed to marshal errand config")

	_, err = file.Write(b)
	Expect(err).NotTo(HaveOccurred())

	return file.Name()
}
