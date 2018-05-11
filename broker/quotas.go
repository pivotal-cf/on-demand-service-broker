package broker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func (b *Broker) checkQuotas(ctx context.Context, plan config.Plan, cfPlanCounts map[cf.ServicePlan]int, serviceOffering string, logger *log.Logger) (error, bool) {
	var quotasErrors []error

	planCounts := convertCfPlanCounts(cfPlanCounts)

	if instanceLimit := plan.Quotas.ServiceInstanceLimit; instanceLimit != nil {
		if err := checkPlanServiceCount(plan, planCounts, *instanceLimit, serviceOffering); err != nil {
			quotasErrors = append(quotasErrors, err)
		}
	}

	if instanceLimit := b.serviceOffering.GlobalQuotas.ServiceInstanceLimit; instanceLimit != nil {
		if err := checkGlobalServiceCount(planCounts, *instanceLimit, serviceOffering); err != nil {
			quotasErrors = append(quotasErrors, err)
		}
	}

	if globalResourceLimits := b.serviceOffering.GlobalQuotas.ResourceLimits; globalResourceLimits != nil {
		if err := checkGlobalResourceQuotaNotExceeded(plan, b.serviceOffering.Plans, planCounts, globalResourceLimits); err != nil {
			quotasErrors = append(quotasErrors, err)
		}
	}

	if planResourceLimits := plan.Quotas.ResourceLimits; planResourceLimits != nil {
		if err := checkPlanResourceQuotaNotExceeded(plan, planCounts, plan.Quotas.ResourceLimits); err != nil {
			quotasErrors = append(quotasErrors, err)
		}
	}

	if len(quotasErrors) > 0 {
		errorStrings := []string{}
		for _, e := range quotasErrors {
			errorStrings = append(errorStrings, e.Error())
		}
		return errors.New(strings.Join(errorStrings, ", ")), false
	}

	return nil, true
}

func convertCfPlanCounts(cfPlanCounts map[cf.ServicePlan]int) map[string]int {
	var brokerPlanCounts = make(map[string]int)

	for plan, count := range cfPlanCounts {
		id := plan.ServicePlanEntity.UniqueID
		brokerPlanCounts[id] = count
	}

	return brokerPlanCounts
}

func checkPlanServiceCount(plan config.Plan, planCounts map[string]int, planInstanceLimit int, serviceOffering string) error {
	count, ok := planCounts[plan.ID]
	if ok && count >= planInstanceLimit {
		return fmt.Errorf("plan instance limit exceeded for service ID: %s. Total instances: %d", serviceOffering, count)
	}
	return nil
}

func checkGlobalServiceCount(planCounts map[string]int, instanceLimit int, serviceOffering string) error {
	var totalServiceInstances = 0
	for _, count := range planCounts {
		totalServiceInstances += count
	}

	if totalServiceInstances >= instanceLimit {
		return fmt.Errorf("global instance limit exceeded for service ID: %s. Total instances: %d", serviceOffering, totalServiceInstances)
	}

	return nil
}

type exceededQuota struct {
	name     string
	limit    int
	usage    int
	required int
}

func checkGlobalResourceQuotaNotExceeded(plan config.Plan, plans []config.Plan, planCounts map[string]int, globalResourceLimits map[string]int) error {
	var exceededQuotas []exceededQuota

	for kind, limit := range globalResourceLimits {
		var currentUsage int

		for _, p := range plans {
			instanceCount := planCounts[plan.ID]
			cost, ok := p.ResourceCosts[kind]
			if ok {
				currentUsage += cost * instanceCount
			}
		}
		required := plan.ResourceCosts[kind]
		if (currentUsage + required) > limit {
			exceededQuotas = append(exceededQuotas, exceededQuota{kind, limit, currentUsage, required})
		}
	}

	if exceededQuotas == nil {
		return nil
	}

	errorDetails := []string{}
	for _, q := range exceededQuotas {
		errorDetails = append(errorDetails, fmt.Sprintf("%s: (limit %d, used %d, requires %d)", q.name, q.limit, q.usage, q.required))
	}

	return fmt.Errorf("global quotas [%s] would be exceeded by this deployment", strings.Join(errorDetails, ", "))
}

func checkPlanResourceQuotaNotExceeded(plan config.Plan, planCounts map[string]int, planResourceLimits map[string]int) error {
	var exceededQuotas []exceededQuota

	for kind, limit := range planResourceLimits {
		var currentUsage int

		instanceCount := planCounts[plan.ID]
		cost, ok := plan.ResourceCosts[kind]
		if ok {
			currentUsage += cost * instanceCount
		}

		if (currentUsage + cost) > limit {
			exceededQuotas = append(exceededQuotas, exceededQuota{kind, limit, currentUsage, cost})
		}
	}

	if exceededQuotas == nil {
		return nil
	}

	errorDetails := []string{}
	for _, q := range exceededQuotas {
		errorDetails = append(errorDetails, fmt.Sprintf("%s: (limit %d, used %d, requires %d)", q.name, q.limit, q.usage, q.required))
	}

	return fmt.Errorf("plan quotas [%s] would be exceeded by this deployment", strings.Join(errorDetails, ", "))
}
