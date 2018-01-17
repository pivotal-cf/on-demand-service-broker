package credhubbroker_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	apifakes "github.com/pivotal-cf/on-demand-service-broker/apiserver/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	credfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

var _ = Describe("CredHub broker", func() {
	var (
		fakeBroker     *apifakes.FakeCombinedBroker
		ctx            context.Context
		instanceID     string
		bindingID      string
		bindDetails    brokerapi.BindDetails
		logBuffer      *bytes.Buffer
		loggerFactory  *loggerfactory.LoggerFactory
		requestIDRegex string
		serviceName    string
	)

	BeforeEach(func() {
		fakeBroker = new(apifakes.FakeCombinedBroker)
		ctx = context.Background()
		instanceID = "ohai"
		bindingID = "rofl"
		bindDetails = brokerapi.BindDetails{
			ServiceID: "big-hybrid-cloud-of-things",
		}
		logBuffer = new(bytes.Buffer)
		loggerFactory = loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "credhubbroker-unit-test", loggerfactory.Flags)
		requestIDRegex = `\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\]`
	})

	Describe("Bind", func() {

		It("returns the credhub reference on Bind", func() {
			creds := "justAString"
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubRef := constructCredhubRef(bindDetails.ServiceID, instanceID, bindingID)
			expectedBindingCredentials := map[string]string{"credhub-ref": credhubRef}

			fakeCredStore := new(credfakes.FakeCredentialStore)
			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)

			bindDetails.AppGUID = "an-app"

			response, err := credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			By("verifying responses of bind")
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Credentials).To(Equal(expectedBindingCredentials))

			By("logging that we are storing credentials")
			brokerctx, _, _, _ := fakeBroker.BindArgsForCall(0)
			requestID := brokercontext.GetReqID(brokerctx)

			Expect(logBuffer.String()).To(SatisfyAll(
				MatchRegexp(requestIDRegex),
				ContainSubstring(
					fmt.Sprintf("storing credentials for instance ID: %s, with binding ID: %s", instanceID, bindingID),
				),
				ContainSubstring(requestID),
			))

			By("receiving correct key & credentials in credstore.Set")
			key, receivedCreds := fakeCredStore.SetArgsForCall(0)
			Expect(key).To(Equal(credhubRef))
			Expect(receivedCreds).To(Equal(creds))
		})

		It("adds permissions to the credentials in the credential store when an app guid exists", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)
			creds := "justAString"
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			appGUID := "app-guid"
			bindDetails.AppGUID = appGUID

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			Expect(fakeCredStore.AddPermissionsCallCount()).To(Equal(1))
			returnedCredentialName, returnedPermissions := fakeCredStore.AddPermissionsArgsForCall(0)

			expectedCredentialName := constructCredhubRef(bindDetails.ServiceID, instanceID, bindingID)
			expectedPermissions := []permissions.Permission{
				permissions.Permission{
					Actor:      fmt.Sprintf("mtls-app:%s", appGUID),
					Operations: []string{"read"},
				},
			}
			Expect(returnedCredentialName).To(Equal(expectedCredentialName))
			Expect(returnedPermissions).To(Equal(expectedPermissions))
		})

		It("adds permissions to the credentials in the credentials store when a credential_client_id exists in the bind resource", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)
			creds := "justAString"
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credentialClientID := "client_id"
			bindDetails.BindResource = &brokerapi.BindResource{
				CredentialClientID: credentialClientID,
			}

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			Expect(fakeCredStore.AddPermissionsCallCount()).To(Equal(1))
			returnedCredentialName, returnedPermissions := fakeCredStore.AddPermissionsArgsForCall(0)

			expectedCredentialName := constructCredhubRef(bindDetails.ServiceID, instanceID, bindingID)
			expectedPermissions := []permissions.Permission{
				permissions.Permission{
					Actor:      fmt.Sprintf("uaa-client:%s", credentialClientID),
					Operations: []string{"read"},
				},
			}

			Expect(returnedCredentialName).To(Equal(expectedCredentialName))
			Expect(returnedPermissions).To(Equal(expectedPermissions))
		})

		It("returns an error when neither app guid or credential_client_id exist in bind request", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)
			creds := "justAString"
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			_, err := credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			Expect(err).To(MatchError(Equal("No app-guid or credential client ID were provided in the binding request, you must configure one of these")))
		})

		It("produces an error if it cannot retrieve the binding from the wrapped broker", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)
			emptyCreds := brokerapi.Binding{}
			fakeBroker.BindReturns(emptyCreds, errors.New("error message from base broker"))

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			receivedCreds, bindErr := credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			Expect(receivedCreds).To(Equal(emptyCreds))
			Expect(bindErr).To(MatchError("error message from base broker"))
		})

		It("produces an error if it cannot store the credential", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)
			bindingResponse := brokerapi.Binding{
				Credentials: "justAString",
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			fakeCredStore.SetReturns(errors.New("credential store unavailable"))
			_, bindErr := credhubBroker.Bind(ctx, instanceID, bindingID, bindDetails)

			Expect(bindErr.Error()).NotTo(ContainSubstring("credential store unavailable"))
			Expect(bindErr.Error()).To(ContainSubstring(instanceID))

			brokerctx, _, _, _ := fakeBroker.BindArgsForCall(0)
			requestID := brokercontext.GetReqID(brokerctx)
			Expect(bindErr.Error()).To(ContainSubstring(requestID))

			Expect(logBuffer.String()).To(ContainSubstring(
				"failed to set credentials in credential store:"))
		})
	})

	Describe("Unbind", func() {
		var unbindDetails = brokerapi.UnbindDetails{
			PlanID:    "asdf",
			ServiceID: "fdsa",
		}

		It("removes the corresponding credentials from the credential store", func() {
			credhubRef := constructCredhubRef(unbindDetails.ServiceID, instanceID, bindingID)

			fakeCredStore := new(credfakes.FakeCredentialStore)
			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)

			err := credhubBroker.Unbind(ctx, instanceID, bindingID, unbindDetails)

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBroker.UnbindCallCount()).To(Equal(1))
			Expect(fakeCredStore.DeleteCallCount()).To(Equal(1))
			Expect(fakeCredStore.DeleteArgsForCall(0)).To(Equal(credhubRef))
			Expect(logBuffer.String()).To(MatchRegexp(requestIDRegex))
			Expect(logBuffer.String()).To(ContainSubstring(
				fmt.Sprintf("removing credentials for instance ID: %s, with binding ID: %s", instanceID, bindingID)))
		})

		It("sets a request ID and passes it through to the broker via the context", func() {
			fakeCredStore := new(credfakes.FakeCredentialStore)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)
			credhubBroker.Unbind(ctx, instanceID, bindingID, unbindDetails)

			brokerctx, _, _, _ := fakeBroker.UnbindArgsForCall(0)
			requestID := brokercontext.GetReqID(brokerctx)

			requestIDRegex = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
			Expect(requestID).To(MatchRegexp(requestIDRegex))
			Expect(logBuffer.String()).To(ContainSubstring(requestID))
		})

		It("returns an error if the wrapped broker unbind call fails", func() {
			baseError := errors.New("foo")
			fakeBroker.UnbindReturns(baseError)
			fakeCredStore := new(credfakes.FakeCredentialStore)
			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)

			err := credhubBroker.Unbind(ctx, instanceID, bindingID, unbindDetails)
			Expect(err).To(MatchError(baseError))
		})

		It("logs a warning if credhub delete call fails", func() {
			credhubError := errors.New("foo")
			fakeCredStore := new(credfakes.FakeCredentialStore)
			fakeCredStore.DeleteReturns(credhubError)
			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore, serviceName, loggerFactory)

			err := credhubBroker.Unbind(ctx, instanceID, bindingID, unbindDetails)
			Expect(err).ToNot(HaveOccurred())
			credhubRef := constructCredhubRef(unbindDetails.ServiceID, instanceID, bindingID)
			Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("WARNING: failed to remove key '%s'", credhubRef)))
		})
	})
})

func constructCredhubRef(serviceID, instanceID, bindingID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", serviceID, instanceID, bindingID)
}
