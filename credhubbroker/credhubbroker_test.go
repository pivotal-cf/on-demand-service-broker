package credhubbroker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	apifakes "github.com/pivotal-cf/on-demand-service-broker/apiserver/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	credfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
)

var _ = Describe("CredHub broker", func() {
	Describe("binding", func() {
		var (
			fakeBroker    *apifakes.FakeCombinedBrokers
			fakeCredStore *credfakes.FakeCredentialStore
			ctx           context.Context
			instanceID    string
			bindingID     string
			details       brokerapi.BindDetails
			credhubKey    string
		)

		BeforeEach(func() {
			fakeBroker = new(apifakes.FakeCombinedBrokers)
			fakeCredStore = new(credfakes.FakeCredentialStore)
			ctx = context.Background()
			instanceID = "ohai"
			bindingID = "rofl"
			details = brokerapi.BindDetails{
				ServiceID: "big-hybrid-cloud-of-things",
			}
			credhubKey = "/c/big-hybrid-cloud-of-things/ohai/rofl/credentials"
		})

		It("stores key-value credentials in CredHub", func() {
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

		It("stores any old muck as credentials in CredHub", func() {
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

		It("stores string credentials in CredHub", func() {
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

		It("passes the return value through from the wrapped broker", func() {
			expectedBindingResponse := brokerapi.Binding{
				Credentials: "anything",
			}
			fakeBroker.BindReturns(expectedBindingResponse, nil)

			credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
			Expect(credhubBroker.Bind(ctx, instanceID, bindingID, details)).To(Equal(expectedBindingResponse))
		})
	})
})
