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

package on_demand_service_broker_test

import sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

var (
	falseVar                           = false
	trueVar                            = true
	serviceID                          = "service-id"
	serviceDescription                 = "the finest service available to humanity"
	serviceBindable                    = false
	servicePlanUpdatable               = true
	serviceMetadataDisplayName         = "service-name"
	serviceMetadataImageURL            = "service-image-url"
	serviceMetaDataLongDescription     = "serviceMetaDataLongDescription"
	serviceMetaDataProviderDisplayName = "serviceMetaDataProviderDisplayName"
	serviceMetaDataDocumentationURL    = "serviceMetaDataDocumentationURL"
	serviceMetaDataSupportURL          = "serviceMetaDataSupportURL"
	serviceMetaDataShareable           = true
	serviceTags                        = []string{"a", "b"}

	dedicatedPlanID           = "dedicated-plan-id"
	dedicatedPlanName         = "dedicated-plan-name"
	dedicatedPlanDisplayName  = "dedicated-plan-display-name"
	dedicatedPlanDescription  = "dedicatedPlanDescription"
	dedicatedPlanCostAmount   = map[string]float64{"usd": 99.0, "eur": 49.0}
	dedicatedPlanCostUnit     = "MONTHLY"
	dedicatedPlanVMType       = "dedicated-plan-vm"
	dedicatedPlanVMExtensions = []string{"what", "an", "extension"}
	dedicatedPlanDisk         = "dedicated-plan-disk"
	dedicatedPlanInstances    = 1
	dedicatedPlanQuota        = 1
	dedicatedPlanNetworks     = []string{"net1"}
	dedicatedPlanAZs          = []string{"az1"}
	dedicatedPlanBullets      = []string{"bullet one", "bullet two", "bullet three"}
	dedicatedPlanUpdateBlock  = &sdk.Update{
		Canaries:        1,
		MaxInFlight:     10,
		CanaryWatchTime: "1000-30000",
		UpdateWatchTime: "1000-30000",
		Serial:          &falseVar,
	}

	highMemoryPlanID           = "high-memory-plan-id"
	highMemoryPlanName         = "high-memory-plan-name"
	highMemoryPlanDescription  = "highMemoryPlanDescription"
	highMemoryPlanVMType       = "high-memory-plan-vm"
	highMemoryPlanVMExtensions = []string{"even", "more", "memory"}
	highMemoryPlanDisplayName  = "dedicated-plan-display-name"
	highMemoryPlanBullets      = []string{"bullet one", "bullet two", "bullet three"}
	highMemoryPlanInstances    = 27
	highMemoryPlanNetworks     = []string{"high1", "high2"}
	highMemoryPlanAZs          = []string{"az1", "az2"}

	spaceGUID        = "space-guid"
	organizationGUID = "organizationGuid"
)
