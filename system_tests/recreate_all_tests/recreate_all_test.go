package recreate_all_test

import (
	"encoding/json"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("The recreate-all errand", func() {

	It("recreates all instances and runs their post-deploy errands", func() {
		boshServiceInstanceName := broker.InstancePrefix + cf.GetServiceInstanceGUID(serviceInstanceName)
		oldVMID := getVMIDForServiceInstance(boshServiceInstanceName)
		Expect(oldVMID).ToNot(BeEmpty(), "unexpected empty vm id")

		session := runRecreateAllErrand(deploymentName)
		Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "recreate all errand failed")

		newVMID := getVMIDForServiceInstance(boshServiceInstanceName)
		Expect(oldVMID).ToNot(Equal(newVMID), "VM was not recreated")

		boshTasks := boshTasksForDeployment(boshServiceInstanceName)
		Expect(boshTasks).To(HaveLen(4), "expected bosh deploy, errand, recreate, errand (reversed)")
		Expect(boshTasks[0].Description).To(HavePrefix("run errand health-check"))
		Expect(boshTasks[0].State).To(Equal("done"))
	})

})

func getVMIDForServiceInstance(boshServiceInstanceName string) string {
	cmd := exec.Command("bosh", "-n", "-d", boshServiceInstanceName, "--json", "vms")
	stdout := gbytes.NewBuffer()
	session, err := gexec.Start(cmd, stdout, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh vms")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "getting VM info failed")

	var boshOutput struct {
		Tables []struct {
			Rows []struct {
				VMCID string `json:"vm_cid"`
			}
		}
	}
	err = json.Unmarshal(stdout.Contents(), &boshOutput)
	Expect(err).NotTo(HaveOccurred(), "Failed unmarshalling json output for VM info")

	return boshOutput.Tables[0].Rows[0].VMCID
}

func runRecreateAllErrand(deploymentName string) *gexec.Session {
	cmd := exec.Command("bosh", "-n", "-d", deploymentName, "run-errand", "recreate-all-service-instances")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run errand")
	return session
}

type boshTask struct {
	Description string `json:"description"`
	State       string `json:"state"`
}

func boshTasksForDeployment(boshServiceInstanceName string) []boshTask {
	cmd := exec.Command("bosh", "-n", "-d", boshServiceInstanceName, "tasks", "--recent", "--json")
	stdout := gbytes.NewBuffer()
	session, err := gexec.Start(cmd, stdout, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred(), "failed to run bosh tasks command")
	Eventually(session, longBOSHTimeout).Should(gexec.Exit(0), "getting tasks failed")

	var boshOutput struct {
		Tables []struct {
			Rows []boshTask
		}
	}
	err = json.Unmarshal(stdout.Contents(), &boshOutput)
	Expect(err).NotTo(HaveOccurred(), "Failed unmarshalling json output for tasks")

	return boshOutput.Tables[0].Rows
}
