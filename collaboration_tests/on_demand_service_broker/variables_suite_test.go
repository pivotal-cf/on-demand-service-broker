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
