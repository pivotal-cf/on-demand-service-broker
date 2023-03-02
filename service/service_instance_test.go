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

package service_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("Service Instance Builder", func() {
	It("creates a CFServiceInstanceLister when SI API is not configured", func() {
		l, _ := service.BuildInstanceLister(new(fakes.FakeCFListerClient), "some-offering-id", config.ServiceInstancesAPI{}, nil)
		Expect(l).To(BeAssignableToTypeOf(&service.CFServiceInstanceLister{}))
	})

	It("creates a ServiceInstanceLister when SI API is configured", func() {
		l, _ := service.BuildInstanceLister(nil, "", config.ServiceInstancesAPI{
			URL: "some-service-instances-api-url.com",
		}, nil)
		Expect(l).To(BeAssignableToTypeOf(&service.ServiceInstanceLister{}))
	})
})
