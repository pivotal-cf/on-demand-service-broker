// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package loggerfactory_test

import (
	"bytes"
	"context"
	"log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

var _ = Describe("Logger Factory", func() {
	It("defines default flags", func() {
		expectedFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC
		Expect(loggerfactory.Flags).To(Equal(expectedFlags))
	})

	It("can create a logger that logs with flags", func() {
		logs := &bytes.Buffer{}
		factory := loggerfactory.New(logs, "some-name", loggerfactory.Flags)

		logger := factory.New()
		logger.Println("some log message")

		Expect(logs.String()).To(MatchRegexp(`\[some-name\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} some log message`))
	})

	It("can create a logger that logs with a request ID", func() {
		logs := &bytes.Buffer{}
		factory := loggerfactory.New(logs, "some-name", 0)

		logger := factory.NewWithRequestID()
		logger.Println("some log message")

		Expect(logs.String()).To(MatchRegexp(`\[some-name\] \[([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\] some log message`))
	})

	Context("can create a logger that uses a context", func() {
		var ctx context.Context

		Context("when request ID is present in context", func() {
			BeforeEach(func() {
				ctx = context.Background()
				ctx = brokercontext.WithReqID(ctx, "some-request-id")
			})

			It("can get the request ID from the context", func() {
				Expect(brokercontext.GetReqID(ctx)).To(Equal("some-request-id"))
			})

			It("can return a logger that logs messages with request ID", func() {
				logs := &bytes.Buffer{}
				factory := loggerfactory.New(logs, "some-name", 0)

				logger := factory.NewWithContext(ctx)
				logger.Println("some log message")

				Expect(logs.String()).To(MatchRegexp(`\[some-name\] \[some-request-id\] some log message`))
			})
		})

		Context("when request ID not present in context", func() {
			BeforeEach(func() {
				ctx = context.Background()
			})

			It("cannot get the request ID from the context", func() {
				Expect(brokercontext.GetReqID(ctx)).To(BeEmpty())
			})

			It("can return a logger that logs messages without request ID", func() {
				logs := &bytes.Buffer{}
				factory := loggerfactory.New(logs, "some-name", 0)

				logger := factory.NewWithContext(ctx)
				logger.Println("some log message")

				Expect(logs.String()).To(MatchRegexp(`\[some-name\] some log message`))
			})
		})
	})
})
