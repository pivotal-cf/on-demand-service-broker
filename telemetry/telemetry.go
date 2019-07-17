package telemetry

import (
	"encoding/json"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	. "github.com/pivotal-cf/on-demand-service-broker/service"
	"log"
	"time"
)

func Build(enableLogging bool, brokerIdentifier string, logger *log.Logger) broker.TelemetryLogger {
	if !enableLogging {
		logger.Printf("Telemetry logging is disabled.")
		return &NoopTelemetryLogger{}
	}

	return &TelemetryLogger{Logger: logger, BrokerIdentifier: brokerIdentifier, Time: &RealTime{format: time.RFC3339}}
}

func (t *TelemetryLogger) LogTotalInstances(instanceLister InstanceLister, item string, operation string) {
	allInstances, err := instanceLister.Instances(nil)
	if err != nil {
		t.Logger.Printf("Failed to query list of instances for telemetry (cause: %s). Skipping total instances log.", err)
	} else {
		t.Logger.Printf(t.buildMessage(allInstances, item, operation))
	}
}

func (t *TelemetryLogger) buildMessage(allInstances []Instance, item string, operation string) string {
	telemetryLog := Log{
		TelemetryTime:   t.Time.Now(),
		TelemetrySource: "odb-" + t.BrokerIdentifier,
		ServiceInstances: ServiceInstances{
			Total: len(allInstances),
		},
		Event: Event{
			Item:      item,
			Operation: operation,
		},
	}
	telemetryMessage, err := json.Marshal(telemetryLog)
	if err != nil {
		t.Logger.Printf("could not marshal telemetry log: %s", err.Error())
	}

	return string(telemetryMessage)
}

//go:generate counterfeiter -o fakes_telemetry/fake_telemetry_time.go . Time
type Time interface {
	Now() string
}

type RealTime struct {
	format string
}

func (r *RealTime) Now() string {
	return time.Now().Format(r.format)
}

type TelemetryLogger struct {
	Logger           *log.Logger
	BrokerIdentifier string
	Time             Time
}

type ServiceInstances struct {
	Total int `json:"total"`
}

type Event struct {
	Item      string `json:"item"`
	Operation string `json:"operation"`
}

type Log struct {
	TelemetryTime    string           `json:"telemetry-time"`
	TelemetrySource  string           `json:"telemetry-source"`
	ServiceInstances ServiceInstances `json:"service-instances"`
	Event            Event            `json:"event"`
}

type NoopTelemetryLogger struct{}

func (t *NoopTelemetryLogger) LogTotalInstances(instanceLister InstanceLister, item string, operation string) {
}
