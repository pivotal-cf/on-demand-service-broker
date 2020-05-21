package register_broker_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"gopkg.in/yaml.v2"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("RegisterBroker", func() {
	var (
		cfServer                                              *ghttp.Server
		serviceBrokersHandler                                 *helpers.FakeHandler
		createBrokerHandler                                   *helpers.FakeHandler
		updateBrokerHandler                                   *helpers.FakeHandler
		servicesHandler                                       *helpers.FakeHandler
		updatePlanHandler                                     *helpers.FakeHandler
		servicePlansOfferingHandler                           *helpers.FakeHandler
		servicePlansVisibilityHandler                         *helpers.FakeHandler
		servicePlansVisibilityDeleteHandler                   *helpers.FakeHandler
		servicePlansVisibilityCreateHandler                   *helpers.FakeHandler
		organizationHandler                                   *helpers.FakeHandler
		brokerName, brokerUsername, brokerPassword, brokerURL string
		serviceID, serviceGUID, planName                      string
		errandConfig                                          config.RegisterBrokerErrandConfig
	)

	BeforeEach(func() {
		brokerName = "some-service-broker"
		brokerURL = "http://example.broker.com"
		brokerUsername = "username"
		brokerPassword = "password"
		serviceGUID = "service-guid"

		serviceID = "a-service"
		planName = "a-plan"

		cfServer = ghttp.NewServer()

		serviceBrokersHandler = new(helpers.FakeHandler)
		createBrokerHandler = new(helpers.FakeHandler)
		updateBrokerHandler = new(helpers.FakeHandler)
		servicesHandler = new(helpers.FakeHandler)
		updatePlanHandler = new(helpers.FakeHandler)
		servicePlansOfferingHandler = new(helpers.FakeHandler)
		servicePlansVisibilityHandler = new(helpers.FakeHandler)
		servicePlansVisibilityDeleteHandler = new(helpers.FakeHandler)
		servicePlansVisibilityCreateHandler = new(helpers.FakeHandler)
		organizationHandler = new(helpers.FakeHandler)

		cfServer.RouteToHandler(http.MethodPost, "/oauth/token", func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte(`{"access_token":"authtoken"}`))
		})
		cfServer.RouteToHandler(http.MethodGet, "/v2/service_brokers", serviceBrokersHandler.Handle)
		cfServer.RouteToHandler(http.MethodPost, "/v2/service_brokers", createBrokerHandler.Handle)
		cfServer.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_brokers/.*`), updateBrokerHandler.Handle)
		cfServer.RouteToHandler(http.MethodGet, "/v2/services", servicesHandler.Handle)
		cfServer.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/services/.*/service_plans`), servicePlansOfferingHandler.Handle)
		cfServer.RouteToHandler(http.MethodGet, "/v2/service_plan_visibilities", servicePlansVisibilityHandler.Handle)
		cfServer.RouteToHandler(http.MethodPost, "/v2/service_plan_visibilities", servicePlansVisibilityCreateHandler.Handle)
		cfServer.RouteToHandler(http.MethodDelete, regexp.MustCompile("/v2/service_plan_visibilities/.*"), servicePlansVisibilityDeleteHandler.Handle)
		cfServer.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_plans/.*`), updatePlanHandler.Handle)
		cfServer.RouteToHandler(http.MethodGet, "/v2/organizations", organizationHandler.Handle)

		errandConfig = config.RegisterBrokerErrandConfig{
			BrokerName:     brokerName,
			BrokerUsername: brokerUsername,
			BrokerPassword: brokerPassword,
			BrokerURL:      brokerURL,
			CF: config.CF{
				URL: cfServer.URL(),
				UAA: config.UAAConfig{
					URL: cfServer.URL(),
					Authentication: config.UAACredentials{
						UserCredentials: config.UserCredentials{Username: "foo", Password: "bar"},
					},
				},
				DisableSSLCertVerification: true,
			},
			ServiceOfferingID: serviceID,
		}
	})

	It("creates a service broker when the broker is not yet registered", func() {
		serviceBrokersHandler.RespondsWith(http.StatusOK, `{"resources":[]}`)
		createBrokerHandler.RespondsWith(http.StatusCreated, "")

		registersBrokerSuccessfully(errandConfig, GinkgoWriter, GinkgoWriter)

		Expect(updateBrokerHandler.RequestsReceived()).To(BeZero(), "update broker was called")
		Expect(createBrokerHandler.RequestsReceived()).To(BeNumerically(">", 0), "no request was made to create broker")
		createRequest := createBrokerHandler.GetRequestForCall(0)
		Expect(createRequest.Body).To(MatchJSON(fmt.Sprintf(`{
				"name": "%s", 
				"broker_url": "%s",
				"auth_username": "%s",
				"auth_password": "%s"
			}`, brokerName, brokerURL, brokerUsername, brokerPassword)))
	})

	It("updates the existing broker when the broker is already registered", func() {
		cfBrokerResponse := fmt.Sprintf(`{
			"resources": [{
				"entity": {"name": "%s"},
				"metadata": {"guid": "%s"}
			}]
		}`, brokerName, serviceGUID)
		serviceBrokersHandler.RespondsWith(http.StatusOK, cfBrokerResponse)
		updateBrokerHandler.RespondsWith(http.StatusOK, "")

		registersBrokerSuccessfully(errandConfig, GinkgoWriter, GinkgoWriter)

		Expect(createBrokerHandler.RequestsReceived()).To(BeZero(), "create broker was called")
		Expect(updateBrokerHandler.RequestsReceived()).To(BeNumerically(">", 0), "no request was made to update broker")

		updateRequest := updateBrokerHandler.GetRequestForCall(0)
		Expect(updateRequest.Body).To(MatchJSON(fmt.Sprintf(`{
				"name": "%s", 
				"broker_url": "%s",
				"auth_username": "%s",
				"auth_password": "%s"
			}`, brokerName, brokerURL, brokerUsername, brokerPassword)))
		Expect(updateRequest.URL).To(Equal("/v2/service_brokers/" + serviceGUID))
	})

	When("there are service plans configured", func() {
		BeforeEach(func() {
			serviceBrokersHandler.RespondsWith(http.StatusOK, `{"resources":[]}`)
			createBrokerHandler.RespondsWith(http.StatusCreated, "")
		})

		It("enables plans that are configured to be enabled", func() {
			enabledPlanGUID := "enabled-unique-uid"

			errandConfig.Plans = []config.PlanAccess{
				{
					Name:            planName,
					CFServiceAccess: config.PlanEnabled,
				},
			}

			servicesHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
			  "resources": [
				{
				  "entity": {
					"unique_id": %q,
					"service_plans_url": "/v2/services/%s/service_plans"
				  }
				}
			  ]
			}`, serviceID, serviceGUID))

			servicePlansOfferingHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
				"resources" : [{
					"entity": {"name": %q},
					"metadata": {"guid": %q}
				}]}`, planName, enabledPlanGUID))

			servicePlansVisibilityHandler.RespondsWith(http.StatusOK, `{"resources": [
				{ "metadata": { "url": "/v2/service_plan_visibilities/d1b5ea55-f354-4f43-b52e-53045747adb9" } },
				{ "metadata": { "url": "/v2/service_plan_visibilities/some-plan-visibility-guid" } }
			]}`)
			servicePlansVisibilityDeleteHandler.RespondsWith(http.StatusNoContent, "")
			updatePlanHandler.RespondsWith(http.StatusCreated, ``)

			registersBrokerSuccessfully(errandConfig, GinkgoWriter, GinkgoWriter)

			servicePlansRequest := updatePlanHandler.GetRequestForCall(0)
			Expect(servicePlansRequest.URL).To(Equal("/v2/service_plans/" + enabledPlanGUID))
			Expect(servicePlansRequest.Body).To(MatchJSON(`{
				"public": true
			}`))
			Expect(servicePlansVisibilityDeleteHandler.RequestsReceived()).To(BeNumerically(">=", 2), "no request was made to service plan visibility")
		})

		It("disables plans that are configured to be disabled", func() {
			disabledPlanGUID := "disabled-unique-uid"
			disabledPlanName := "disabled-plan-name"
			errandConfig.Plans = []config.PlanAccess{
				{
					Name:            disabledPlanName,
					CFServiceAccess: config.PlanDisabled,
				},
			}
			servicesHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
			  "resources": [
				{
				  "entity": {
					"unique_id": %q,
					"service_plans_url": "/v2/services/%s/service_plans"
				  }
				}
			  ]
			}`, serviceID, serviceGUID))

			servicePlansOfferingHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
				"resources" : [{
					"entity": {"name": %q},
					"metadata": {"guid": %q}
				}]}`, disabledPlanName, disabledPlanGUID))

			servicePlansVisibilityHandler.RespondsWith(http.StatusOK, `{"resources": [
				{ "metadata": { "url": "/v2/service_plan_visibilities/d1b5ea55-f354-4f43-b52e-53045747adb9" } },
				{ "metadata": { "url": "/v2/service_plan_visibilities/some-plan-visibility-guid" } }
			]}`)
			servicePlansVisibilityDeleteHandler.RespondsWith(http.StatusNoContent, "")
			updatePlanHandler.RespondsWith(http.StatusCreated, ``)

			registersBrokerSuccessfully(errandConfig, GinkgoWriter, GinkgoWriter)

			servicePlansRequest := updatePlanHandler.GetRequestForCall(0)
			Expect(servicePlansRequest.URL).To(Equal("/v2/service_plans/" + disabledPlanGUID))
			Expect(servicePlansRequest.Body).To(MatchJSON(`{
				"public": false
			}`))
			Expect(servicePlansVisibilityDeleteHandler.RequestsReceived()).To(BeNumerically(">=", 2), "no request was made to service plan visibility")
		})

		It("restricts plans to specified orgs for plans that are configured to be org-restricted", func() {
			orgRestrictedPlanGUID := "disabled-unique-uid"
			orgRestrictedPlanName := "disabled-plan-name"
			orgGUID := "some-org-guid"

			errandConfig.Plans = []config.PlanAccess{
				{
					Name:            orgRestrictedPlanName,
					CFServiceAccess: config.PlanOrgRestricted,
				},
			}
			organizationHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
			  "resources": [
				{
				  "metadata": {
					"guid": "%s"
				  }
				}
			  ]
			}`, orgGUID))

			servicesHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
			  "resources": [
				{
				  "entity": {
					"unique_id": %q,
					"service_plans_url": "/v2/services/%s/service_plans"
				  }
				}
			  ]
			}`, serviceID, serviceGUID))

			servicePlansOfferingHandler.RespondsWith(http.StatusOK, fmt.Sprintf(`{
				"resources" : [{
					"entity": {"name": %q},
					"metadata": {"guid": %q}
				}]}`, orgRestrictedPlanName, orgRestrictedPlanGUID))

			servicePlansVisibilityHandler.RespondsWith(http.StatusOK, `{"resources": [
				{ "metadata": { "url": "/v2/service_plan_visibilities/d1b5ea55-f354-4f43-b52e-53045747adb9" } },
				{ "metadata": { "url": "/v2/service_plan_visibilities/some-plan-visibility-guid" } }
			]}`)
			servicePlansVisibilityDeleteHandler.RespondsWith(http.StatusNoContent, "")
			updatePlanHandler.RespondsWith(http.StatusCreated, ``)
			servicePlansVisibilityCreateHandler.RespondsWith(http.StatusCreated, ``)

			registersBrokerSuccessfully(errandConfig, GinkgoWriter, GinkgoWriter)

			servicePlansRequest := updatePlanHandler.GetRequestForCall(0)
			Expect(servicePlansRequest.URL).To(Equal("/v2/service_plans/" + orgRestrictedPlanGUID))
			Expect(servicePlansRequest.Body).To(MatchJSON(`{
				"public": false
			}`))
			Expect(servicePlansVisibilityDeleteHandler.RequestsReceived()).To(BeNumerically(">=", 2), "no request was made to service plan visibility")

			createVisibilityRequest := servicePlansVisibilityCreateHandler.GetRequestForCall(0)
			Expect(createVisibilityRequest.Body).To(MatchJSON(fmt.Sprintf(`{
 				"service_plan_guid": "%s",
				"organization_guid": "%s"
			}`, orgRestrictedPlanGUID, orgGUID)))
		})

	})

	Describe("error handling", func() {
		It("fails when config path is not specified", func() {
			cmd := exec.Command(binaryPath)

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1), "succeeded unexpectedly")
			Expect(session).To(gbytes.Say("-configPath must be given as argument"))
		})

		It("fails when config path is not a file", func() {
			cmd := exec.Command(binaryPath, "-configPath", "not-a-file")

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1), "succeeded unexpectedly")
			Expect(session).To(gbytes.Say("error reading file -configPath"))
		})

		It("fails when running the errand fails", func() {
			serviceBrokersHandler.RespondsWith(http.StatusInternalServerError, "")

			session := executeBinary(errandConfig, GinkgoWriter, GinkgoWriter)
			Expect(session).To(gexec.Exit(1))
		})
	})

})

func registersBrokerSuccessfully(errandConfig config.RegisterBrokerErrandConfig, stdout, stderr io.Writer) *gexec.Session {
	session := executeBinary(errandConfig, stdout, stderr)
	Expect(session).To(gexec.Exit(0))

	return session
}

func executeBinary(errandConfig config.RegisterBrokerErrandConfig, stdout, stderr io.Writer) *gexec.Session {
	errandConfigPath, err := ioutil.TempFile("/tmp", "")
	Expect(err).ToNot(HaveOccurred())

	b, err := yaml.Marshal(errandConfig)
	Expect(err).ToNot(HaveOccurred())

	_, err = errandConfigPath.Write(b)
	Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command(binaryPath, "-configPath", errandConfigPath.Name())

	session, err := gexec.Start(cmd, stdout, stderr)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session, 5*time.Second).Should(gexec.Exit())
	Expect(os.RemoveAll(errandConfigPath.Name())).To(Succeed())

	return session
}
