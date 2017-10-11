package credhubbroker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	apifakes "github.com/pivotal-cf/on-demand-service-broker/apiserver/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	credfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
)

var _ = Describe("CredHub broker", func() {

	var (
		fakeBroker *apifakes.FakeCombinedBrokers
		ctx        context.Context
		instanceID string
		bindingID  string
		details    brokerapi.BindDetails
		credhubKey string
	)

	BeforeEach(func() {
		fakeBroker = new(apifakes.FakeCombinedBrokers)
		ctx = context.Background()
		instanceID = "ohai"
		bindingID = "rofl"
		details = brokerapi.BindDetails{
			ServiceID: "big-hybrid-cloud-of-things",
		}
		credhubKey = "/c/big-hybrid-cloud-of-things/ohai/rofl/credentials"
	})

	It("passes the return value through from the wrapped broker", func() {
		expectedBindingResponse := brokerapi.Binding{
			Credentials: "anything",
		}
		fakeBroker.BindReturns(expectedBindingResponse, nil)

		fakeCredStore := new(credfakes.FakeCredentialStore)
		credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
		Expect(credhubBroker.Bind(ctx, instanceID, bindingID, details)).To(Equal(expectedBindingResponse))
	})

	Describe("binding with generic credstore", func() {

		var (
			fakeCredStore *credfakes.FakeCredentialStore
		)

		BeforeEach(func() {
			fakeCredStore = new(credfakes.FakeCredentialStore)
		})

		It("stores key-value credentials in cred store", func() {
			creds := map[string]interface{}{
				"foo": 42,
				"bar": "IPA",
			}
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
			credhubBroker.Bind(ctx, instanceID, bindingID, details)
			key, receivedCreds := fakeCredStore.SetArgsForCall(0)

			Expect(key).To(Equal(credhubKey))
			Expect(receivedCreds).To(Equal(creds))
		})

		It("stores anything as credentials in cred store", func() {
			creds := []interface{}{1, 2, "foo", map[string]interface{}{"foo": 42}}
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
			credhubBroker.Bind(ctx, instanceID, bindingID, details)

			key, receivedCreds := fakeCredStore.SetArgsForCall(0)
			Expect(key).To(Equal(credhubKey))
			Expect(receivedCreds).To(Equal(creds))
		})

		It("stores string credentials in cred store", func() {
			creds := "justAString"
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
			credhubBroker.Bind(ctx, instanceID, bindingID, details)

			key, receivedCreds := fakeCredStore.SetArgsForCall(0)
			Expect(key).To(Equal(credhubKey))
			Expect(receivedCreds).To(Equal(creds))
		})

		XContext("when cannot store credentials in credhub", func() {
			It("calls unbind on the wrapped broker", func() {

			})

			It("returns an error", func() {

			})
		})
	})

	Describe("binding with CredHub store", func() {
		It("sets credentials on the credhub api", func() {
			var method, path string
			var body []byte

			wait := make(chan struct{})
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method = r.Method
				path = r.URL.Path
				body, _ = ioutil.ReadAll(r.Body)
				w.Write([]byte(`{}`))
				close(wait)
			})
			fakeCredHubServer := httptest.NewServer(handler)
			credHubStore := &credhubbroker.CredHubStore{
				APIURL:       fakeCredHubServer.URL,
				ClientID:     "",
				ClientSecret: "",
			}
			creds := map[string]interface{}{
				"foo": 42,
				"bar": "IPA",
			}
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)
			credhubBroker := credhubbroker.New(fakeBroker, credHubStore)
			_, err := credhubBroker.Bind(ctx, instanceID, bindingID, details)
			Expect(err).ToNot(HaveOccurred())

			Eventually(wait).Should(BeClosed())
			Expect(method).To(Equal("PUT"))
			Expect(path).To(Equal("/api/v1/data"))
			marshalledCreds, _ := json.Marshal(creds)
			Expect(body).To(MatchJSON(fmt.Sprintf(`
			{
			  "name": "%s",
			  "type": "json",
			  "overwrite": false,
			  "value": %s
			}`, credhubKey, marshalledCreds)))
		})

		It("returns an error if the request to the API fails", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{}`))
			})
			fakeCredHubServer := httptest.NewServer(handler)
			credHubStore := &credhubbroker.CredHubStore{
				APIURL:       fakeCredHubServer.URL,
				ClientID:     "",
				ClientSecret: "",
			}
			creds := map[string]interface{}{
				"foo": 42,
				"bar": "IPA",
			}
			bindingResponse := brokerapi.Binding{
				Credentials: creds,
			}
			fakeBroker.BindReturns(bindingResponse, nil)
			credhubBroker := credhubbroker.New(fakeBroker, credHubStore)
			_, err := credhubBroker.Bind(ctx, instanceID, bindingID, details)
			Expect(err).To(HaveOccurred())
		})
	})
})
