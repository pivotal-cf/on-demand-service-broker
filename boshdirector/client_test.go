package boshdirector_test

import (
	"log"

	. "github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"

	"errors"

	"net/http"

	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("New", func() {
	var (
		fakeHTTPClient           *fakes.FakeNetworkDoer
		fakeAuthenticatorBuilder *fakes.FakeAuthenticatorBuilder
		fakeAuthHeaderBuilder    *fakes.FakeAuthHeaderBuilder
		fakeCertAppender         *fakes.FakeCertAppender
	)

	BeforeEach(func() {
		fakeHTTPClient = new(fakes.FakeNetworkDoer)
		fakeAuthenticatorBuilder = new(fakes.FakeAuthenticatorBuilder)
		fakeAuthHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
		fakeCertAppender = new(fakes.FakeCertAppender)

		fakeCertAppender.AppendCertsFromPEMReturns(true)
	})

	It("returns a bosh client that works", func() {
		fakeHTTPClient.DoReturns(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(`{"user_authentication": { "options": {"url": "some-url"}}}`)),
			StatusCode: http.StatusOK,
		}, nil)

		fakeAuthHeaderBuilder.AddAuthHeaderStub = func(req *http.Request, logger *log.Logger) error {
			req.Header.Set("Authorization", "Bearer unit-test-token")
			return nil
		}
		fakeAuthenticatorBuilder.NewAuthHeaderBuilderReturns(fakeAuthHeaderBuilder, nil)

		client, err := New("http://example.org", true, []byte("a totally trustworthy cert"), fakeHTTPClient, fakeAuthenticatorBuilder, fakeCertAppender, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(client).NotTo(BeNil())

		By("getting bosh info")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   "/info",
				Method: "GET",
				Host:   "example.org",
			}, 0))
		Expect(fakeHTTPClient.DoArgsForCall(0).Header.Get("Authorization")).To(Equal(""))

		By("making an authenticator")
		Expect(fakeAuthenticatorBuilder.NewAuthHeaderBuilderCallCount()).To(Equal(1))
		info, disableSSLCertVerification := fakeAuthenticatorBuilder.NewAuthHeaderBuilderArgsForCall(0)
		Expect(info.UserAuthentication.Options.URL).To(Equal("some-url"))
		Expect(disableSSLCertVerification).To(BeTrue(), "SSL Certificate Verification should be skipped here")

		By("appending the trusted certificate to the system cert pool")
		Expect(fakeCertAppender.AppendCertsFromPEMCallCount()).To(Equal(1))
		Expect(fakeCertAppender.AppendCertsFromPEMArgsForCall(0)).To(Equal([]byte("a totally trustworthy cert")))

		By("finally returning a client with a sensible PollingInterval that we can use for a working GetInfo call")
		Expect(client.PollingInterval).To(BeEquivalentTo(5))

		client.GetInfo(logger)
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   "/info",
				Method: "GET",
				Host:   "example.org",
				Header: http.Header{
					"Authorization": []string{"Bearer unit-test-token"},
				},
			}, 1))
	})

	It("returns an error when it can't retrieve bosh info", func() {
		fakeHTTPClient.DoReturns(nil, errors.New("didn't work"))
		_, err := New("example.org", true, []byte("a totally trustworthy cert"), fakeHTTPClient, fakeAuthenticatorBuilder, fakeCertAppender, logger)
		Expect(err).To(MatchError(ContainSubstring("error fetching BOSH director information: error reaching bosh director: didn't work")))
	})

	It("returns an error when it can't build bosh authenticator", func() {
		fakeHTTPClient.DoReturns(&http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(`{}`)),
			StatusCode: http.StatusOK,
		}, nil)

		fakeAuthenticatorBuilder.NewAuthHeaderBuilderReturns(nil, errors.New("oh noes!"))

		_, err := New("example.org", true, []byte("a totally trustworthy cert"), fakeHTTPClient, fakeAuthenticatorBuilder, fakeCertAppender, logger)
		Expect(err).To(MatchError(ContainSubstring("error creating BOSH authorization header builder: oh noes!")))
	})
})
