package telemetry

import (
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	. "github.com/pivotal-cf/on-demand-service-broker/service"
	"log"
)

func Build(enableLogging bool, brokerIdentifier string, logger *log.Logger) broker.TelemetryLogger {
	if !enableLogging {
		return &NoopTelemetryLogger{}
	}

	return &TelemetryLogger{logger: logger, brokerIdentifier: brokerIdentifier}
}

type TelemetryLogger struct {
	logger           *log.Logger
	brokerIdentifier string
}

func (t *TelemetryLogger) LogTotalInstances(instanceLister InstanceLister, operation string) {
	allInstances, err := instanceLister.Instances(nil)
	if err != nil {
		t.logger.Printf("Failed to query list of instances for telemetry (cause: %s). Skipping total instances log.", err)
	} else {
		t.logger.Printf(`{"telemetry-source":"odb-%s","service-instances":{"total":%d,"operation":%q}}`, t.brokerIdentifier, len(allInstances), operation)
	}
}

type NoopTelemetryLogger struct {
}

func (t *NoopTelemetryLogger) LogTotalInstances(instanceLister InstanceLister, operation string) {
}
