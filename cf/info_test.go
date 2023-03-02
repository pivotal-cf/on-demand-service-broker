package cf_test

import (
	"fmt"
	"io"
	"log"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
)

var _ = Describe("info", func() {
	var server *mockhttp.Server
	var testLogger *log.Logger
	var logBuffer *gbytes.Buffer
	var authHeaderBuilder *fakes.FakeAuthHeaderBuilder

	const (
		cfAuthorizationHeader = "auth-header"
		serviceGUID           = "06df08f9-5a58-4d33-8097-32d0baf3ce1e"
	)

	BeforeEach(func() {
		authHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
		authHeaderBuilder.AddAuthHeaderStub = func(req *http.Request, logger *log.Logger) error {
			req.Header.Set("Authorization", cfAuthorizationHeader)
			return nil
		}
		server = mockcfapi.New()
		logBuffer = gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	AfterEach(func() {
		server.VerifyMocks()
	})

	Describe("GetAPIVersion", func() {
		It("gets cloudfoundry api version", func() {
			server.VerifyAndMock(
				mockcfapi.GetInfo().RespondsOKWith(
					`{
					  "name": "",
					  "build": "",
					  "support": "http://support.cloudfoundry.com",
					  "version": 0,
					  "description": "",
					  "authorization_endpoint": "https://login.services-enablement-bosh-lite-aws.cf-app.com",
					  "token_endpoint": "https://uaa.services-enablement-bosh-lite-aws.cf-app.com",
					  "min_cli_version": null,
					  "min_recommended_cli_version": null,
					  "api_version": "2.57.0",
					  "app_ssh_endpoint": "ssh.services-enablement-bosh-lite-aws.cf-app.com:2222",
					  "app_ssh_host_key_fingerprint": "a6:d1:08:0b:b0:cb:9b:5f:c4:ba:44:2a:97:26:19:8a",
					  "app_ssh_oauth_client": "ssh-proxy",
					  "logging_endpoint": "wss://loggregator.services-enablement-bosh-lite-aws.cf-app.com:443",
					  "doppler_logging_endpoint": "wss://doppler.services-enablement-bosh-lite-aws.cf-app.com:4443"
					}`,
				),
			)
			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.GetAPIVersion(testLogger)).To(Equal("2.57.0"))
		})

		It("fails, if get info fails", func() {
			server.VerifyAndMock(
				mockcfapi.GetInfo().RespondsInternalServerErrorWith("nothing today, thank you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, getVersionErr := client.GetAPIVersion(testLogger)
			Expect(getVersionErr.Error()).To(ContainSubstring("nothing today, thank you"))
		})
	})

	Describe("CheckMinimumOSBAPIVersion", func() {
		When("the specified minimum version is invalid", func() {
			It("logs an error and returns false", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.CheckMinimumOSBAPIVersion("", testLogger)).To(BeFalse())
				Expect(logBuffer).To(gbytes.Say("error parsing specified OSBAPI version '' to semver:"))

				Expect(client.CheckMinimumOSBAPIVersion("not-semver", testLogger)).To(BeFalse())
				Expect(logBuffer).To(gbytes.Say("error parsing specified OSBAPI version 'not-semver' to semver:"))
			})
		})

		When("the API returns an error", func() {
			It("logs an error and returns false", func() {
				server.VerifyAndMock(
					mockcfapi.GetInfo().RespondsInternalServerErrorWith(""),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.CheckMinimumOSBAPIVersion("1.2.3", testLogger)).To(BeFalse())
				Expect(logBuffer).To(gbytes.Say("error requesting OSBAPI version: Unexpected reponse status 500"))
			})
		})

		When("the API returns an invalid version", func() {
			It("logs an error and returns false", func() {
				server.VerifyAndMock(
					mockcfapi.GetInfo().RespondsOKWith(`{"osbapi_version": "nonsemver"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.CheckMinimumOSBAPIVersion("1.2.3", testLogger)).To(BeFalse())
				Expect(logBuffer).To(gbytes.Say("error parsing discovered OSBAPI version 'nonsemver' to semver:"))
			})
		})

		DescribeTable(
			"correctly checking minuimum OSBABI versions",
			func(required, actual string, result bool) {
				server.VerifyAndMock(
					mockcfapi.GetInfo().RespondsOKWith(fmt.Sprintf(`{"osbapi_version": "%s"}`, actual)),
				)
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.CheckMinimumOSBAPIVersion(required, testLogger)).To(Equal(result), fmt.Sprintf("%s >= %s = %t", actual, required, result))
			},
			Entry("3 digit above", "1.2.3", "1.2.4", true),
			Entry("3 digit same", "1.2.3", "1.2.3", true),
			Entry("3 digit below", "1.2.3", "1.2.2", false),
			Entry("2 digit above", "1.21", "1.22", true),
			Entry("2 digit same", "1.21", "1.21", true),
			Entry("2 digit below", "1.21", "1.11", false),
			Entry("1 digit above", "1.21", "2", true),
			Entry("1 digit same", "1.21", "1", false),
			Entry("1 digit below", "1.21", "0", false),
		)
	})
})
