// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package create_test

import (
	"flag"
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	instances int
	interval  int
	service   string
	plan      string
)

//These can be run using `ginkgo -- -instances 0 -interval 0 -service foo -plan bar`
func init() {
	flag.IntVar(&instances, "instances", 0, "the number of instances to create")
	flag.IntVar(&interval, "interval", 0, "the interval between create service requests")
	flag.StringVar(&service, "service", "", "the service offering")
	flag.StringVar(&plan, "plan", "", "the service plan")
	flag.Parse()

	if service == "" || plan == "" {
		log.Fatal("service & plan must be set.")
	}

	if instances == 0 {
		log.Fatal("instances must be set.")
	}
}

func TestLoadTesting(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Suite")
}
