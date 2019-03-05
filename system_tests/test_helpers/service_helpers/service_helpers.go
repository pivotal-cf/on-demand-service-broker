package service_helpers

type ServiceType int

const (
	Redis ServiceType = iota
	Kafka
)

func (s ServiceType) GetServiceOpsFile() string {
	switch s {
	case Redis:
		return "redis.yml"
	case Kafka:
		return "kafka.yml"
	}
	return ""
}
