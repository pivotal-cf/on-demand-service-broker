package instanceiterator_test

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/broker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pkg/errors"
)

var _ = Describe("CfTriggerer", func() {
	Describe("TriggerOperation", func() {
		var (
			fakeCFClient            *fakes.FakeCFClient
			expectedMaintenanceInfo cf.MaintenanceInfo
		)

		BeforeEach(func() {
			fakeCFClient = new(fakes.FakeCFClient)
			expectedMaintenanceInfo = cf.MaintenanceInfo{
				Version: "2.1.3",
			}
			fakeCFClient.GetPlanByServiceInstanceGUIDReturns(cf.ServicePlan{
				ServicePlanEntity: cf.ServicePlanEntity{
					MaintenanceInfo: expectedMaintenanceInfo,
				},
			}, nil)
			fakeCFClient.UpgradeServiceInstanceReturns(cf.LastOperation{
				Type:  cf.OperationType("update"),
				State: cf.OperationStateInProgress,
			}, nil)
		})

		It("should return operation type accepted when CF responds with state in progress", func() {
			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			triggeredOperation, _ := cfTriggerer.TriggerOperation(
				service.Instance{
					GUID:         "service-instance-id",
					PlanUniqueID: "plan-id",
				})

			Expect(fakeCFClient.GetPlanByServiceInstanceGUIDCallCount()).To(Equal(1), "expected to get CF plan")

			Expect(fakeCFClient.UpgradeServiceInstanceCallCount()).To(Equal(1), "expected to call CF upgrade service")
			_, actualMaintenanceInfo, _ := fakeCFClient.UpgradeServiceInstanceArgsForCall(0)
			Expect(actualMaintenanceInfo).To(Equal(expectedMaintenanceInfo))

			Expect(triggeredOperation.State).To(Equal(instanceiterator.OperationAccepted))
		})

		It("should return operation type failed when CF responds with state failed", func() {
			fakeCFClient.UpgradeServiceInstanceReturns(cf.LastOperation{
				Type:  cf.OperationType("update"),
				State: cf.OperationStateFailed,
			}, nil)

			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			triggeredOperation, _ := cfTriggerer.TriggerOperation(service.Instance{
				GUID:         "service-instance-id",
				PlanUniqueID: "plan-id",
			})

			Expect(fakeCFClient.GetPlanByServiceInstanceGUIDCallCount()).To(Equal(1), "expected to get CF plan")

			Expect(fakeCFClient.UpgradeServiceInstanceCallCount()).To(Equal(1), "expected to call CF upgrade service")
			_, actualMaintenanceInfo, _ := fakeCFClient.UpgradeServiceInstanceArgsForCall(0)
			Expect(actualMaintenanceInfo).To(Equal(expectedMaintenanceInfo))

			Expect(triggeredOperation.State).To(Equal(instanceiterator.OperationFailed))
		})

		It("should return operation type skipped when CF responds with state succeeded", func() {
			fakeCFClient.UpgradeServiceInstanceReturns(cf.LastOperation{
				Type:  cf.OperationType("update"),
				State: cf.OperationStateSucceeded,
			}, nil)

			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			triggeredOperation, _ := cfTriggerer.TriggerOperation(service.Instance{
				GUID:         "service-instance-id",
				PlanUniqueID: "plan-id",
			})

			Expect(fakeCFClient.GetPlanByServiceInstanceGUIDCallCount()).To(Equal(1), "expected to get CF plan")

			Expect(fakeCFClient.UpgradeServiceInstanceCallCount()).To(Equal(1), "expected to call CF upgrade service")
			_, actualMaintenanceInfo, _ := fakeCFClient.UpgradeServiceInstanceArgsForCall(0)
			Expect(actualMaintenanceInfo).To(Equal(expectedMaintenanceInfo))

			Expect(triggeredOperation.State).To(Equal(instanceiterator.OperationSkipped))
		})

		It("should return an error when the CF client cannot get plan by unique ID", func() {
			fakeCFClient.GetPlanByServiceInstanceGUIDReturns(cf.ServicePlan{}, errors.New("failed to get plan"))
			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			_, err := cfTriggerer.TriggerOperation(service.Instance{
				GUID:         "service-instance-id",
				PlanUniqueID: "plan-id",
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get plan"))
		})

		It("should return an error when the CF client cannot upgrade service instance", func() {
			fakeCFClient.UpgradeServiceInstanceReturns(cf.LastOperation{}, errors.New("failed to upgrade instance"))
			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			_, err := cfTriggerer.TriggerOperation(service.Instance{
				GUID:         "service-instance-id",
				PlanUniqueID: "plan-id",
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to upgrade instance"))
		})
	})

	Describe("Check", func() {
		var (
			fakeCFClient *fakes.FakeCFClient
		)
		BeforeEach(func() {
			fakeCFClient = new(fakes.FakeCFClient)
		})

		It("should return latest state of the service instance", func() {
			fakeCFClient.GetLastOperationForInstanceReturns(
				cf.LastOperation{
					Type:  cf.OperationType("update"),
					State: cf.OperationStateSucceeded,
				},
				nil)

			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			expectedServiceInstanceGUID := "service-instance-id"
			triggeredOperation, _ := cfTriggerer.Check(expectedServiceInstanceGUID, broker.OperationData{})

			Expect(fakeCFClient.GetLastOperationForInstanceCallCount()).To(Equal(1), "expected to call CF get service instance")

			actualServiceInstanceGUID, _ := fakeCFClient.GetLastOperationForInstanceArgsForCall(0)
			Expect(actualServiceInstanceGUID).To(Equal(expectedServiceInstanceGUID))

			Expect(triggeredOperation.State).To(Equal(instanceiterator.OperationSucceeded))
		})

		It("return an error when the CF client cannot get service instance", func() {
			fakeCFClient.GetLastOperationForInstanceReturns(
				cf.LastOperation{},
				errors.New("failed to get service instance"))
			cfTriggerer := instanceiterator.NewCFTrigger(fakeCFClient, new(log.Logger))

			_, err := cfTriggerer.Check("service-instance-id", broker.OperationData{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get service instance"))
		})
	})
})
