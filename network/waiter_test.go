package network_test

import (
	"errors"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"github.com/pivotal-cf/on-demand-service-broker/network/fakes"
)

var _ = Describe("Waiter", func() {
	var fakeLookUpper *fakes.FakeHostLookUpper
	var fakeSleeper *fakes.FakeSleeper

	var subject network.HostWaiter

	BeforeEach(func() {
		fakeLookUpper = new(fakes.FakeHostLookUpper)
		fakeSleeper = new(fakes.FakeSleeper)

		subject = network.HostWaiter{HostLookUpper: fakeLookUpper.Spy, Sleeper: fakeSleeper.Spy}
	})

	It("checks if the host is available using the lookuper", func() {
		actualError := subject.Wait("gopher://user:pass@foobar.com/address?fo=ba#12", 0, 0)

		Expect(fakeLookUpper.CallCount()).To(Equal(1))
		Expect(fakeLookUpper.ArgsForCall(0)).To(Equal("foobar.com"))
		Expect(actualError).ToNot(HaveOccurred())
	})

	It("returns an error if the host is not available within given retries", func() {
		fakeLookUpper.Returns(nil, errors.New("no such host"))

		actualError := subject.Wait("gopher://user:pass@foobar.com/address?fo=ba#12", 0, 10)
		Expect(fakeLookUpper.CallCount()).To(Equal(11))
		Expect(actualError).To(HaveOccurred())
	})

	It("does not retry for errors other than 'no such host'", func() {
		fakeLookUpper.Returns(nil, errors.New("no internet"))

		actualError := subject.Wait("gopher://user:pass@foobar.com/address?fo=ba#12", 1000, 1000)
		Expect(fakeLookUpper.CallCount()).To(Equal(1))
		Expect(fakeSleeper.CallCount()).To(Equal(0))
		Expect(actualError).To(HaveOccurred())
	})

	It("returns an error if the url is not valid", func() {
		actualError := subject.Wait("3.com:1", 0, 0)
		Expect(fakeLookUpper.CallCount()).To(Equal(0))
		Expect(actualError).To(HaveOccurred())
	})

	It("performs an exponential backoff when it fails to contact host", func() {
		fakeLookUpper.Returns(nil, errors.New("no such host"))
		actualError := subject.Wait("http://valid.url/", 10, 5)

		Expect(actualError).To(HaveOccurred())

		tenMs := 10 * time.Millisecond
		Expect(fakeSleeper.CallCount()).To(Equal(5))
		Expect(fakeSleeper.ArgsForCall(0)).To(Equal(tenMs))
		Expect(fakeSleeper.ArgsForCall(1)).To(Equal(2 * tenMs))
		Expect(fakeSleeper.ArgsForCall(2)).To(Equal(4 * tenMs))
		Expect(fakeSleeper.ArgsForCall(3)).To(Equal(8 * tenMs))
		Expect(fakeSleeper.ArgsForCall(4)).To(Equal(16 * tenMs))
	})

	It("can succeed after a few failures", func() {
		fakeLookUpper.Returns(nil, errors.New("no such host"))
		fakeLookUpper.ReturnsOnCall(4, nil, nil)
		actualError := subject.Wait("http://valid.url/", 10, 10)
		Expect(actualError).NotTo(HaveOccurred())
		Expect(fakeLookUpper.CallCount()).To(Equal(5))
	})
})
