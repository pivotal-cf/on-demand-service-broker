package mgmtapi

import "fmt"

type Metric struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type BrokerMetrics struct {
	serviceOfferingName string
	metrics             []Metric
}

func (m BrokerMetrics) AddPlanMetric(serviceOfferingPlanName, metricName string, value int) BrokerMetrics {
	return m.addMetric(Metric{
		Key:   fmt.Sprintf("/on-demand-broker/%s/%s/%s", m.serviceOfferingName, serviceOfferingPlanName, metricName),
		Unit:  "count",
		Value: float64(value),
	})
}

func (m BrokerMetrics) AddGlobalMetric(metricName string, value int) BrokerMetrics {
	return m.addMetric(Metric{
		Key:   fmt.Sprintf("/on-demand-broker/%s/%s", m.serviceOfferingName, metricName),
		Unit:  "count",
		Value: float64(value),
	})
}

func (m BrokerMetrics) addMetric(metric Metric) BrokerMetrics {
	return BrokerMetrics{
		serviceOfferingName: m.serviceOfferingName,
		metrics:             append(m.metrics, metric),
	}
}
