// Package billing productizes usage metering into plans, invoices and a
// provider-neutral payment face (RB§29 §46 R10, D2-P3-1 / todo93). Aggregates
// come from the usage_records collector (todo71); domestic CN channels stay
// behind the same PaymentProvider interface as stripe-class providers.
package billing

import (
	"fmt"
	"time"
)

// Tiers mirror the organizations.tier CHECK constraint (schema v1).
const (
	TierFree = "free"
	TierPro  = "pro"
)

// OveragePolicy is deliberate product behavior: exceeding included quota
// throttles new work but NEVER deletes customer data.
const OveragePolicy = "throttle-only"

// Plan is one priced offering tier.
type Plan struct {
	Tier                 string
	MonthlyBaseCents     int64
	IncludedBuildMinutes float64
	IncludedCPUCoreHours float64
	OverageCPUCentsHour  int64
	MaxBuildConcurrency  int
}

var catalog = map[string]Plan{
	TierFree: {
		Tier: TierFree, MonthlyBaseCents: 0,
		IncludedBuildMinutes: 200, IncludedCPUCoreHours: 10,
		OverageCPUCentsHour: 0, MaxBuildConcurrency: 1,
	},
	TierPro: {
		Tier: TierPro, MonthlyBaseCents: 1900,
		IncludedBuildMinutes: 2000, IncludedCPUCoreHours: 120,
		OverageCPUCentsHour: 8, MaxBuildConcurrency: 4,
	},
}

// PlanFor resolves a tier onto its priced plan definition.
func PlanFor(tier string) (Plan, error) {
	p, ok := catalog[tier]
	if !ok {
		return Plan{}, fmt.Errorf("%w: %q", ErrUnknownTier, tier)
	}
	return p, nil
}

// BillingPeriod returns the UTC calendar-month window containing now.
func BillingPeriod(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
