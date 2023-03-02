// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package boshdirector_test

import (
	"errors"

	boshdir "github.com/cloudfoundry/bosh-cli/v7/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/v7/uaa"
	. "github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("New", func() {
	var (
		fakeCertAppender                          *fakes.FakeCertAppender
		fakeDirector, fakeDirectorUnauthenticated *fakes.FakeDirector
		fakeDirectorFactory                       *fakes.FakeDirectorFactory
		fakeUAAFactory                            *fakes.FakeUAAFactory
	)

	BeforeEach(func() {
		fakeCertAppender = new(fakes.FakeCertAppender)
		fakeDirectorFactory = new(fakes.FakeDirectorFactory)
		fakeDirectorUnauthenticated = new(fakes.FakeDirector)
		fakeDirector = new(fakes.FakeDirector)
		fakeUAAFactory = new(fakes.FakeUAAFactory)

		fakeCertAppender.AppendCertsFromPEMReturns(true)

		fakeDirectorFactory.NewReturnsOnCall(0, fakeDirectorUnauthenticated, nil)
		fakeDirectorFactory.NewReturnsOnCall(1, fakeDirector, nil)

		fakeDirector.IsAuthenticatedReturns(true, nil)
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
				[]byte("a totally trustworthy cert"),
				fakeCertAppender,
				fakeDirectorFactory,
				fakeUAAFactory,
				boshAuthConfig,
				fakeDNSRetrieverFactory.Spy,
				fakeBoshHTTPFactory.Spy,
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

			By("appending the trusted certificate to the system cert pool")
			Expect(fakeCertAppender.AppendCertsFromPEMCallCount()).To(Equal(1))
			Expect(fakeCertAppender.AppendCertsFromPEMArgsForCall(0)).To(Equal([]byte("a totally trustworthy cert")))

			By("finally returning a client with a sensible PollingInterval that we can use for a working GetInfo call")
			Expect(client.PollingInterval).To(BeEquivalentTo(5))

			By("ensuring that the client works")
			err = client.VerifyAuth(logger)
			Expect(err).NotTo(HaveOccurred())

			By("having configured uaa")
			Expect(fakeUAAFactory.NewCallCount()).To(Equal(1))
			uaaConfig := fakeUAAFactory.NewArgsForCall(0)
			Expect(uaaConfig).To(Equal(boshuaa.Config{
				Host:         "uaa.url.example.com",
				Port:         12345,
				CACert:       "a totally trustworthy cert",
				Client:       boshAuthConfig.UAA.ClientCredentials.ID,
				ClientSecret: boshAuthConfig.UAA.ClientCredentials.Secret,
			}))

		})

		Describe("but New fails", func() {
			It("errors when bosh url is not valid", func() {
				_, err := New(
					"https://not a valid url",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)
				Expect(err).To(MatchError(ContainSubstring("Failed to build director config from url")))
			})

			It("errors when the director factory errors", func() {
				fakeDirectorFactory.NewReturnsOnCall(0, new(fakes.FakeDirector), errors.New("could not build director"))
				_, err := New(
					"https://example.org:25666",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)
				Expect(err).To(MatchError(ContainSubstring("Failed to build director: could not build director")))
			})

			It("errors when the director fails to GetInfo", func() {
				fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{}, errors.New("could not get info"))
				_, err := New(
					"https://example.org:25666",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)

				Expect(err).To(MatchError(ContainSubstring("error fetching BOSH director information: could not get info")))
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

				client, err := New(
					"https://example.org:25666",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)
				Expect(err).NotTo(HaveOccurred())
				err = client.VerifyAuth(logger)
				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA config from url")))
			})

			It("errors when uaa is not deployed", func() {
				fakeDirectorUnauthenticated.InfoReturns(boshdir.Info{
					Version: "1.3262.0.0 (00000000)",
					Auth: boshdir.UserAuthentication{
						Type: "basic",
					},
				}, nil)

				client, err := New(
					"https://example.org:25666",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)
				Expect(err).NotTo(HaveOccurred())
				err = client.VerifyAuth(logger)
				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA config from url: Expected non-empty UAA URL")))
			})

			It("errors when uaa factory returns an error", func() {
				fakeUAAFactory.NewReturns(new(fakes.FakeUAA), errors.New("failed to build uaa"))
				client, err := New(
					"https://example.org:25666",
					[]byte("a totally trustworthy cert"),
					fakeCertAppender,
					fakeDirectorFactory,
					fakeUAAFactory,
					boshAuthConfig,
					fakeDNSRetrieverFactory.Spy,
					fakeBoshHTTPFactory.Spy,
					logger,
				)
				Expect(err).NotTo(HaveOccurred())
				err = client.VerifyAuth(logger)
				Expect(err).To(MatchError(ContainSubstring("Failed to build UAA client: failed to build uaa")))
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
			basicAuthConfig := config.Authentication{
				Basic: config.UserCredentials{Username: "example-username", Password: "example-password"},
			}
			client, err := New(
				"http://example.org:25666",
				[]byte("a totally trustworthy cert"),
				fakeCertAppender,
				fakeDirectorFactory,
				fakeUAAFactory,
				basicAuthConfig,
				fakeDNSRetrieverFactory.Spy,
				fakeBoshHTTPFactory.Spy,
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
