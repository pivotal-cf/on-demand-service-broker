package service

type Instance struct {
	GUID     string `json:"service_instance_id"`
	PlanGUID string `json:"plan_id"`
}
