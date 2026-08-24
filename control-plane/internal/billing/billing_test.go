package billing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPlanCatalog(t *testing.T) {
	free, ferr := PlanFor(TierFree)
	if ferr != nil || free.MonthlyBaseCents != 0 {
		t.Fatalf("free plan: %v %v", free, ferr)
	}
	pro, perr := PlanFor(TierPro)
	if perr != nil || pro.MaxBuildConcurrency <= free.MaxBuildConcurrency {
		t.Fatalf("pro must outrank free: %v %v", pro, perr)
	}
	if _, err := PlanFor("gold"); !errors.Is(err, ErrUnknownTier) {
		t.Fatalf("unknown tier: %v", err)
	}
}

func TestBillingPeriodBounds(t *testing.T) {
	start, end := BillingPeriod(time.Date(2026, 8, 24, 13, 5, 0, 0, time.UTC))
	wantStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("period = %s..%s", start, end)
	}
}

func TestAggregateInvoiceWithinIncluded(t *testing.T) {
	pro, _ := PlanFor(TierPro)
	start, end := BillingPeriod(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	inv := AggregateInvoice("o1", pro, start, end, 500, 40)
	if len(inv.Lines) != 1 || inv.Lines[0].Kind != "base-pro" {
		t.Fatalf("lines = %+v", inv.Lines)
	}
	if inv.TotalCents != pro.MonthlyBaseCents {
		t.Fatalf("total = %d", inv.TotalCents)
	}
}

func TestAggregateInvoiceOverageAndThrottle(t *testing.T) {
	pro, _ := PlanFor(TierPro)
	start, _ := BillingPeriod(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	end := start.AddDate(0, 1, 0)
	inv := AggregateInvoice("o1", pro, start, end, 2500, 130.5)
	var cpuLine, throttleLine bool
	for _, l := range inv.Lines {
		switch l.Kind {
		case "overage-cpu-core-hour":
			cpuLine = true
			if l.Quantity != 10.5 || l.AmountCents != 84 {
				t.Fatalf("cpu line = %+v", l)
			}
		case "throttled-build-minutes":
			throttleLine = true
			if l.AmountCents != 0 {
				t.Fatalf("throttle must be free: %+v", l)
			}
		}
	}
	if !cpuLine || !throttleLine {
		t.Fatalf("missing lines: %+v", inv.Lines)
	}
	if inv.TotalCents != pro.MonthlyBaseCents+84 {
		t.Fatalf("total = %d", inv.TotalCents)
	}
}

func TestUpgradeThenDowngradeImmediate(t *testing.T) {
	store := NewTierStore(TierFree)
	prov := MockProvider{}
	ctx := context.Background()

	apply := func(tier string) {
		raw, mErr := json.Marshal(map[string]string{
			"org_id": "o1", "from": store.Get(), "to": tier,
		})
		if mErr != nil {
			t.Fatal(mErr)
		}
		ch, wErr := prov.HandleWebhook(ctx, raw)
		if wErr != nil {
			t.Fatalf("%s webhook: %v", tier, wErr)
		}
		if ch.From != store.Get() {
			t.Fatalf("stale from=%q want %q", ch.From, store.Get())
		}
		store.Set(ch.To)
	}

	apply(TierPro)
	if store.Get() != TierPro {
		t.Fatalf("upgrade not immediate: %q", store.Get())
	}
	apply(TierFree)
	if store.Get() != TierFree {
		t.Fatalf("downgrade not immediate: %q", store.Get())
	}
	if _, err := prov.HandleWebhook(ctx,
		[]byte(`{"org_id":"o1","to":"gold"}`)); !errors.Is(err, ErrUnknownTier) {
		t.Fatalf("unknown tier must reject: %v", err)
	}
	if ref, err := prov.CreateCheckout(ctx, "o1", TierPro); err != nil ||
		ref != "mock-checkout-o1-pro" {
		t.Fatalf("checkout = %q %v", ref, err)
	}
}
