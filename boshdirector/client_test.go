package boshdirector_test

import (
	"errors"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	. "github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	"log"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("New", func() {
	var (
		fakeHTTPClient                            *fakes.FakeNetworkDoer
		fakeAuthenticatorBuilder                  *fakes.FakeAuthenticatorBuilder
		fakeAuthHeaderBuilder                     *fakes.FakeAuthHeaderBuilder
		fakeCertAppender                          *fakes.FakeCertAppender
		fakeDirector, fakeDirectorUnauthenticated *fakes.FakeDirector
		fakeDirectorFactory                       *fakes.FakeDirectorFactory
		fakeUAAFactory                            *fakes.FakeUAAFactory
		fakeUAA                                   *fakes.FakeUAA
	)

	BeforeEach(func() {
		fakeHTTPClient = new(fakes.FakeNetworkDoer)
		fakeAuthenticatorBuilder = new(fakes.FakeAuthenticatorBuilder)
		fakeAuthHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
		fakeCertAppender = new(fakes.FakeCertAppender)
		fakeDirectorFactory = new(fakes.FakeDirectorFactory)
		fakeDirectorUnauthenticated = new(fakes.FakeDirector)
		fakeDirector = new(fakes.FakeDirector)
		fakeUAA = new(fakes.FakeUAA)
		fakeUAAFactory = new(fakes.FakeUAAFactory)

		fakeCertAppender.AppendCertsFromPEMReturns(true)

		fakeDirectorFactory.NewReturnsOnCall(0, fakeDirectorUnauthenticated, nil)
		fakeDirectorFactory.NewReturnsOnCall(1, fakeDirector, nil)

		fakeDirector.IsAuthenticatedReturns(true, nil)

		fakeAuthHeaderBuilder.AddAuthHeaderStub = func(req *http.Request, logger *log.Logger) error {
			req.Header.Set("Authorization", "Bearer unit-test-token")
			return nil
		}
		fakeAuthenticatorBuilder.NewAuthHeaderBuilderReturns(fakeAuthHeaderBuilder, nil)
	})

	Context("when UAA is configured", func() {
		BeforeEach(func() {
			fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{
				Version: "1.3262.0.0 (00000000)",
				Auth: boshdir.UserAuthentication{
					Type: "uaa",
					Options: map[string]interface{}{
						"url": "uaa.url.example.com:12345",
					},
				},
			}, nil)
			fakeDirector.InfoReturns(boshdir.Info{
				Version: "1.3262.0.0 (00000000)",
				User:    "bosh-username",
				Auth: boshdir.UserAuthentication{
					Type: "uaa",
					Options: map[string]interface{}{
						"url": "uaa.url.example.com:12345",
					},
				},
			}, nil)
		})
		It("returns a bosh client that works", func() {
			client, err := New(
				"http://example.org:25666",
				true,
				[]byte("a totally trustworthy cert"),
				fakeHTTPClient,
				fakeAuthenticatorBuilder,
				fakeCertAppender,
				fakeDirectorFactory,
				fakeUAAFactory,
				boshAuthConfig,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			By("getting bosh info from the non-authenticated director")
			directorConfig, taskReporter, fileReporter := fakeDirectorFactory.NewArgsForCall(0)
			Expect(directorConfig).To(Equal(boshdir.FactoryConfig{
				Host:   "example.org",
				Port:   25666,
				CACert: "a totally trustworthy cert",
			}))
			Expect(directorConfig.TokenFunc).To(BeNil())
			Expect(taskReporter).To(Equal(boshdir.NoopTaskReporter{}))
			Expect(fileReporter).To(Equal(boshdir.NoopFileReporter{}))
			Expect(fakeDirectorUnauthenticated.InfoCallCount()).To(Equal(1))

			By("configuring uaa")
			Expect(fakeUAAFactory.NewCallCount()).To(Equal(1))
			uaaConfig := fakeUAAFactory.NewArgsForCall(0)
			Expect(uaaConfig).To(Equal(boshuaa.Config{
				Host:         "uaa.url.example.com",
				Port:         12345,
				CACert:       "a totally trustworthy cert",
				Client:       boshAuthConfig.UAA.ID,
				ClientSecret: boshAuthConfig.UAA.Secret,
			}))

			By("building an authenticated client")
			directorConfig, taskReporter, fileReporter = fakeDirectorFactory.NewArgsForCall(1)
			Expect(directorConfig.Host).To(Equal("example.org"))
			Expect(directorConfig.Port).To(Equal(25666))
			Expect(directorConfig.CACert).To(Equal("a totally trustworthy cert"))
			Expect(directorConfig.TokenFunc).NotTo(BeNil())
			Expect(taskReporter).To(Equal(boshdir.NoopTaskReporter{}))
			Expect(fileReporter).To(Equal(boshdir.NoopFileReporter{}))

			By("making an authenticator")
			Expect(fakeAuthenticatorBuilder.NewAuthHeaderBuilderCallCount()).To(Equal(1))
			UAAURL, disableSSLCertVerification := fakeAuthenticatorBuilder.NewAuthHeaderBuilderArgsForCall(0)
			Expect(UAAURL).To(Equal("uaa.url.example.com:12345"))
			Expect(disableSSLCertVerification).To(BeTrue(), "SSL Certificate Verification should be skipped here")

			By("appending the trusted certificate to the system cert pool")
			Expect(fakeCertAppender.AppendCertsFromPEMCallCount()).To(Equal(1))
			Expect(fakeCertAppender.AppendCertsFromPEMArgsForCall(0)).To(Equal([]byte("a totally trustworthy cert")))

			By("finally returning a client with a sensible PollingInterval that we can use for a working GetInfo call")
			Expect(client.PollingInterval).To(BeEquivalentTo(5))

			By("ensuring that the client works")
			err = client.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("but New fails", func() {
			It("errors when bosh url is not valid", func() {
				_, err := New(
					"https://not a valid url",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)
				Expect(err).To(MatchError(ContainSubstring("Failed to build director config from url")))
			})

			It("errors when the director factory errors", func() {
				fakeDirectorFactory.NewReturnsOnCall(0, new(fakes.FakeDirector), errors.New("could not build director"))
				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)
				Expect(err).To(MatchError(ContainSubstring("Failed to build unauthenticated director client: could not build director")))
			})

			It("errors when the director fails to GetInfo", func() {
				fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{}, errors.New("could not get info"))
				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("error fetching BOSH director information: could not get info")))
			})

			It("errors when the director fails to build the authorization header builder", func() {
				fakeAuthenticatorBuilder.NewAuthHeaderBuilderReturns(new(fakes.FakeAuthHeaderBuilder), errors.New("could not build authheader"))
				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("Failed to create BOSH authorization header builder: could not build authheader")))
			})

			It("errors when uaa url is not valid", func() {
				fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{
					Version: "1.3262.0.0 (00000000)",
					Auth: boshdir.UserAuthentication{
						Type: "uaa",
						Options: map[string]interface{}{
							"url": "http://what is this",
						},
					},
				}, nil)

				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA config from url")))
			})

			It("errors when uaa is not deployed", func() {
				fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{
					Version: "1.3262.0.0 (00000000)",
					Auth: boshdir.UserAuthentication{
						Type: "basic",
					},
				}, nil)

				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA config from url: Expected non-empty UAA URL")))
			})

			It("errors when uaa factory returns an error", func() {
				fakeUAAFactory.NewReturns(new(fakes.FakeUAA), errors.New("failed to build uaa"))
				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA client: failed to build uaa")))
			})

			It("errors when authenticated director fails to build", func() {
				fakeDirectorFactory.NewReturnsOnCall(1, new(fakes.FakeDirector), errors.New("failed to build director"))
				_, err := New(
					"https://example.org:25666",
					true,
					[]byte("a totally trustworthy cert"),
					fakeHTTPClient,
					fakeAuthenticatorBuilder,
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("Failed to build authenticated director client: failed to build director")))
			})
		})
	})

	Context("when UAA is not configured (a.k.a. Basic auth)", func() {
		BeforeEach(func() {
			fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{
				Version: "1.3262.0.0 (00000000)",
				Auth: boshdir.UserAuthentication{
					Type: "basic",
				},
			}, nil)
			fakeDirector.InfoReturns(boshdir.Info{
				Version: "1.3262.0.0 (00000000)",
				User:    "bosh-username",
				Auth: boshdir.UserAuthentication{
					Type: "basic",
				},
			}, nil)
		})
		It("returns a bosh client that works", func() {
			basicAuthConfig := config.BOSHAuthentication{
				Basic: config.UserCredentials{Username: "example-username", Password: "example-password"},
			}
			client, err := New(
				"http://example.org:25666",
				true,
				[]byte("a totally trustworthy cert"),
				fakeHTTPClient,
				fakeAuthenticatorBuilder,
				fakeCertAppender,
				fakeDirectorFactory,
				fakeUAAFactory,
				basicAuthConfig,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			By("getting bosh info from the non-authenticated director")
			directorConfig, taskReporter, fileReporter := fakeDirectorFactory.NewArgsForCall(0)
			Expect(directorConfig).To(Equal(boshdir.FactoryConfig{
				Host:   "example.org",
				Port:   25666,
				CACert: "a totally trustworthy cert",
			}))
			Expect(directorConfig.TokenFunc).To(BeNil())
			Expect(taskReporter).To(Equal(boshdir.NoopTaskReporter{}))
			Expect(fileReporter).To(Equal(boshdir.NoopFileReporter{}))
			Expect(fakeDirectorUnauthenticated.InfoCallCount()).To(Equal(1))

			By("not configuring uaa")
			Expect(fakeUAAFactory.NewCallCount()).To(Equal(0))

			By("building an authenticated client")
			directorConfig, taskReporter, fileReporter = fakeDirectorFactory.NewArgsForCall(1)
			Expect(directorConfig.Host).To(Equal("example.org"))
			Expect(directorConfig.Port).To(Equal(25666))
			Expect(directorConfig.CACert).To(Equal("a totally trustworthy cert"))
			Expect(directorConfig.Client).To(Equal(basicAuthConfig.Basic.Username))
			Expect(directorConfig.ClientSecret).To(Equal(basicAuthConfig.Basic.Password))
			Expect(taskReporter).To(Equal(boshdir.NoopTaskReporter{}))
			Expect(fileReporter).To(Equal(boshdir.NoopFileReporter{}))

			By("making an authenticator")
			Expect(fakeAuthenticatorBuilder.NewAuthHeaderBuilderCallCount()).To(Equal(1))
			UAAURL, disableSSLCertVerification := fakeAuthenticatorBuilder.NewAuthHeaderBuilderArgsForCall(0)
			Expect(UAAURL).To(Equal(""))
			Expect(disableSSLCertVerification).To(BeTrue(), "SSL Certificate Verification should be skipped here")

			By("appending the trusted certificate to the system cert pool")
			Expect(fakeCertAppender.AppendCertsFromPEMCallCount()).To(Equal(1))
			Expect(fakeCertAppender.AppendCertsFromPEMArgsForCall(0)).To(Equal([]byte("a totally trustworthy cert")))

			By("finally returning a client with a sensible PollingInterval that we can use for a working GetInfo call")
			Expect(client.PollingInterval).To(BeEquivalentTo(5))

			By("ensuring that the client works")
			err = client.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
