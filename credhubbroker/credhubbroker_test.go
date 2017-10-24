package credhubbroker_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	apifakes "github.com/pivotal-cf/on-demand-service-broker/apiserver/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	credfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
)

var _ = Describe("CredHub broker", func() {
	var (
		fakeBroker *apifakes.FakeCombinedBroker
		ctx        context.Context
		instanceID string
		bindingID  string
		details    brokerapi.BindDetails
	)

	BeforeEach(func() {
		fakeBroker = new(apifakes.FakeCombinedBroker)
		ctx = context.Background()
		instanceID = "ohai"
		bindingID = "rofl"
		details = brokerapi.BindDetails{
			ServiceID: "big-hybrid-cloud-of-things",
		}
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

	It("stores credentials and constructs the key", func() {
		fakeCredStore := new(credfakes.FakeCredentialStore)
		creds := "justAString"
		bindingResponse := brokerapi.Binding{
			Credentials: creds,
		}
		fakeBroker.BindReturns(bindingResponse, nil)

		credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
		credhubBroker.Bind(ctx, instanceID, bindingID, details)

		credhubKey := fmt.Sprintf("/c/%s/%s/%s/credentials", details.ServiceID, instanceID, bindingID)
		key, receivedCreds := fakeCredStore.SetArgsForCall(0)
		Expect(key).To(Equal(credhubKey))
		Expect(receivedCreds).To(Equal(creds))
	})

	It("produces an error if it cannot retrieve the binding from the wrapped broker", func() {
		fakeCredStore := new(credfakes.FakeCredentialStore)
		emptyCreds := brokerapi.Binding{}
		fakeBroker.BindReturns(emptyCreds, errors.New("unable to create binding"))

		credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
		receivedCreds, bindErr := credhubBroker.Bind(ctx, instanceID, bindingID, details)

		Expect(receivedCreds).To(Equal(emptyCreds))
		Expect(bindErr).To(MatchError("unable to create binding"))
	})

	It("produces an error if it cannot store the credential", func() {
		fakeCredStore := new(credfakes.FakeCredentialStore)
		creds := "justAString"
		bindingResponse := brokerapi.Binding{
			Credentials: creds,
		}
		fakeBroker.BindReturns(bindingResponse, nil)

		credhubBroker := credhubbroker.New(fakeBroker, fakeCredStore)
		fakeCredStore.SetReturns(errors.New("unable to set credentials in credential store"))
		_, bindErr := credhubBroker.Bind(ctx, instanceID, bindingID, details)

		Expect(bindErr).To(MatchError("unable to set credentials in credential store"))
	})
})
