package telemetry

import (
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	. "github.com/pivotal-cf/on-demand-service-broker/service"
	"log"
)

func Build(enableLogging bool, logger *log.Logger) broker.TelemetryLogger {
	if !enableLogging {
		return &NoopTelemetryLogger{}
	}

	return &TelemetryLogger{logger: logger}
}

type TelemetryLogger struct {
	logger *log.Logger
}

func (t *TelemetryLogger) LogTotalInstances(instanceLister InstanceLister, brokerIdentifier, operation string) {
	allInstances, err := instanceLister.Instances(nil)
	if err != nil {
		t.logger.Printf("Failed to query list of instances for telemetry (cause: %s). Skipping total instances log.", err)
	} else {
		t.logger.Printf(`{"telemetry-source":"odb-%s","service-instances":{"total":%d,"operation":%q}}`, brokerIdentifier, len(allInstances), operation)
	}
}

type NoopTelemetryLogger struct {
}

func (t *NoopTelemetryLogger) LogTotalInstances(instanceLister InstanceLister, brokerIdentifier, operation string) {
}
