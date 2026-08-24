package billing

import (
	"math"
	"time"
)

// InvoiceLine is one priced row of a monthly statement.
type InvoiceLine struct {
	Kind        string  `json:"kind"`
	Quantity    float64 `json:"quantity"`
	UnitCents   int64   `json:"unit_cents"`
	AmountCents int64   `json:"amount_cents"`
}

// Invoice aggregates one organization's billing period.
type Invoice struct {
	OrgID       string        `json:"org_id"`
	PeriodStart time.Time     `json:"period_start"`
	PeriodEnd   time.Time     `json:"period_end"`
	Lines       []InvoiceLine `json:"lines"`
	TotalCents  int64         `json:"total_cents"`
}

// AggregateInvoice prices a period from usage_records-derived totals: the
// plan base always bills; CPU core-hours beyond inclusion bill per the
// overage rate while excess build minutes surface as a zero-cost throttle
// note (data is never deleted per OveragePolicy).
func AggregateInvoice(
	orgID string, plan Plan, start, end time.Time, buildMinutes, cpuCoreHours float64,
) Invoice {
	inv := Invoice{OrgID: orgID, PeriodStart: start, PeriodEnd: end}
	inv.Lines = append(inv.Lines, InvoiceLine{
		Kind: "base-" + plan.Tier, Quantity: 1,
		UnitCents: plan.MonthlyBaseCents, AmountCents: plan.MonthlyBaseCents,
	})
	if extra := cpuCoreHours - plan.IncludedCPUCoreHours; extra > 0 && plan.OverageCPUCentsHour > 0 {
		q := round2(extra)
		inv.Lines = append(inv.Lines, InvoiceLine{
			Kind: "overage-cpu-core-hour", Quantity: q,
			UnitCents:   plan.OverageCPUCentsHour,
			AmountCents: int64(q * float64(plan.OverageCPUCentsHour)),
		})
	}
	if extraMin := buildMinutes - plan.IncludedBuildMinutes; extraMin > 0 {
		inv.Lines = append(inv.Lines, InvoiceLine{
			Kind: "throttled-build-minutes", Quantity: round2(extraMin),
		})
	}
	for _, l := range inv.Lines {
		inv.TotalCents += l.AmountCents
	}
	return inv
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
