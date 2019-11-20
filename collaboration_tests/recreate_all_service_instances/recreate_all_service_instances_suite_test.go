package recreate_all_service_instances_test

import (
	"os"
	"testing"

	"github.com/onsi/gomega/gbytes"

	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	credhubfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	manifestsecretsfakes "github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
	serviceadapterfakes "github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	taskfakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"

	"github.com/pivotal-cf/on-demand-service-broker/collaboration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestRecreateAllServiceInstances(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecreateAllServiceInstances Suite")
}

var (
	pathToRecreateAll   string
	fakeCommandRunner   *serviceadapterfakes.FakeCommandRunner
	fakeTaskBoshClient  *taskfakes.FakeBoshClient
	fakeTaskBulkSetter  *taskfakes.FakeBulkSetter
	fakeCfClient        *fakes.FakeCloudFoundryClient
	fakeBoshClient      *fakes.FakeBoshClient
	fakeCredentialStore *credhubfakes.FakeCredentialStore
	fakeCredhubOperator *manifestsecretsfakes.FakeCredhubOperator
	loggerBuffer        *gbytes.Buffer
)

var _ = BeforeSuite(func() {
	var err error
	pathToRecreateAll, err = gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/recreate-all-service-instances")
	Expect(err).ToNot(HaveOccurred(), "unexpected error when building the binary")
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func StartServer(conf config.Config) *helpers.Server {
	fakeCommandRunner = new(serviceadapterfakes.FakeCommandRunner)
	fakeTaskBulkSetter = new(taskfakes.FakeBulkSetter)
	fakeCredentialStore = new(credhubfakes.FakeCredentialStore)
	fakeCredhubOperator = new(manifestsecretsfakes.FakeCredhubOperator)
	loggerBuffer = gbytes.NewBuffer()
	stopServer := make(chan os.Signal)

	server, err := helpers.StartServer(conf, stopServer, fakeCommandRunner, fakeTaskBoshClient, fakeTaskBulkSetter, fakeCfClient, fakeBoshClient, new(fakes.FakeHasher), fakeCredentialStore, fakeCredhubOperator, loggerBuffer)
	Expect(err).NotTo(HaveOccurred())
	return server
}
