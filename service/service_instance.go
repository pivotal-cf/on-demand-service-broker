package service

type Instance struct {
	GUID         string `json:"service_instance_id"`
	PlanUniqueID string `json:"plan_id"`
}
