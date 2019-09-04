package telemetry

import (
	"encoding/json"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/service"
	"log"
	"time"
)

//go:generate counterfeiter -o fakes_telemetry/fake_telemetry_time.go . Time
type Time interface {
	Now() string
}

type RealTime struct {
	format string
}

type ServiceInstances struct {
	Total int `json:"total"`
}

type ServiceInstancesPerPlan struct {
	PlanID string `json:"plan-id"`
	Total  int    `json:"total"`
}

type Event struct {
	Item      string `json:"item"`
	Operation string `json:"operation"`
}

type TotalInstancesLog struct {
	TelemetryTime    string           `json:"telemetry-time"`
	TelemetrySource  string           `json:"telemetry-source"`
	ServiceInstances ServiceInstances `json:"service-instances"`
	Event            Event            `json:"event"`
}

type PerPlanInstancesLog struct {
	TelemetryTime           string                  `json:"telemetry-time"`
	TelemetrySource         string                  `json:"telemetry-source"`
	ServiceInstancesPerPlan ServiceInstancesPerPlan `json:"service-instances-per-plan"`
	Event                   Event                   `json:"event"`
}

type TelemetryLogger struct {
	logger             *log.Logger
	brokerIdentifier   string
	time               Time
	brokerServicePlans config.Plans
}

func Build(enableLogging bool, serviceOffering config.ServiceOffering, logger *log.Logger) broker.TelemetryLogger {
	if !enableLogging {
		logger.Printf("Telemetry logging is disabled.")
		return &NoopTelemetryLogger{}
	}

	return NewTelemetryLogger(logger, serviceOffering, &RealTime{format: time.RFC3339})
}

func NewTelemetryLogger(logger *log.Logger, serviceOffering config.ServiceOffering, timer Time) broker.TelemetryLogger {
	return &TelemetryLogger{
		logger:             logger,
		brokerIdentifier:   "odb-" + serviceOffering.Name,
		brokerServicePlans: serviceOffering.Plans,
		time:               timer,
	}
}

func (t *TelemetryLogger) LogInstances(instanceLister InstanceLister, item string, operation string) {
	allInstances, err := instanceLister.Instances(nil)
	if err != nil {
		t.logger.Printf("Failed to query list of instances for telemetry (cause: %s). Skipping total instances log.", err)
	} else {
		t.logTotalInstances(allInstances, Event{Item: item, Operation: operation})
		t.logInstancesPerPlan(allInstances, Event{Item: item, Operation: operation})
	}
}

func (t *TelemetryLogger) logTotalInstances(allInstances []Instance, event Event) {
	telemetryLog := TotalInstancesLog{
		TelemetryTime:   t.time.Now(),
		TelemetrySource: t.brokerIdentifier,
		ServiceInstances: ServiceInstances{
			Total: len(allInstances),
		},
		Event: event,
	}

	t.logger.Printf(t.marshalLog(telemetryLog))
}

func (t *TelemetryLogger) logInstancesPerPlan(instances []Instance, event Event) {
	instancesPerPlan := map[string]int{}

	for _, instance := range instances {
		instancesPerPlan[instance.PlanUniqueID]++
	}

	for _, plan := range t.brokerServicePlans {
		count := instancesPerPlan[plan.ID]
		planInstancesLog := PerPlanInstancesLog{
			TelemetryTime:   t.time.Now(),
			TelemetrySource: t.brokerIdentifier,
			Event:           event,
			ServiceInstancesPerPlan: ServiceInstancesPerPlan{
				PlanID: plan.ID,
				Total:  count,
			},
		}

		t.logger.Printf(t.marshalLog(planInstancesLog))
	}
}

func (t *TelemetryLogger) marshalLog(telemetryLog interface{}) string {
	telemetryMessage, err := json.Marshal(telemetryLog)
	if err != nil {
		t.logger.Printf("could not marshal telemetry log: %s", err.Error())
	}

	return string(telemetryMessage)
}

type NoopTelemetryLogger struct{}

func (r *RealTime) Now() string {
	return time.Now().Format(r.format)
}

func (t *NoopTelemetryLogger) LogInstances(instanceLister InstanceLister, item string, operation string) {
}
