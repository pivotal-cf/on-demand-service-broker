// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task_test

import (
	"bytes"
	"io"
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

const (
	existingPlanID                = "some-plan-id"
	existingPlanName              = "some-plan-name"
	existingPlanInstanceGroupName = "instance-group-name"
	secondPlanID                  = "another-plan"
	serviceOfferingID             = "service-id"
	deploymentName                = "some-deployment-name"
)

var (
	logBuffer     *bytes.Buffer
	loggerFactory *loggerfactory.LoggerFactory
	logger        *log.Logger
)

var _ = BeforeEach(func() {
	logBuffer = new(bytes.Buffer)
	loggerFactory = loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "task-unit-tests", log.LstdFlags)
	logger = loggerFactory.NewWithRequestID()
})

func TestTask(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Task Suite")
}
