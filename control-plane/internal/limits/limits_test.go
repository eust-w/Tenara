package limits

import (
	"testing"
	"time"
)

var base = time.Unix(1700000000, 0)

func TestTokenBudgetExhaustsThenRefills(t *testing.T) {
	rl := NewRateLimiter(3, 3)
	for i := 0; i < 3; i++ {
		if ok, _ := rl.Allow("tok", base.Add(time.Duration(i)*time.Second)); !ok {
			t.Fatalf("request %d must pass within budget", i+1)
		}
	}
	if ok, wait := rl.Allow("tok", base.Add(4*time.Second)); ok || wait <= 0 {
		t.Fatalf("over-budget request must be rejected with a wait hint")
	}
	if ok, _ := rl.Allow("tok", base.Add(61*time.Second)); !ok {
		t.Fatal("bucket must refill after a minute")
	}
}

func TestKeysAreIsolated(t *testing.T) {
	rl := NewRateLimiter(1, 60)
	if ok, _ := rl.Allow("token-a", base); !ok {
		t.Fatal("a must pass")
	}
	if ok, _ := rl.Allow("token-b", base); !ok {
		t.Fatal("b must have its own bucket")
	}
}

func TestBuildGateSerializesSameOrg(t *testing.T) {
	gate := NewBuildGate(BuildConcurrencyFree)
	ok, _ := gate.TryAcquire("org-a")
	if !ok {
		t.Fatal("first build must be admitted")
	}
	ok2, wait := gate.TryAcquire("org-a")
	if ok2 || wait <= 0 {
		t.Fatalf("second concurrent build must wait (wait=%s)", wait)
	}
	gate.Release("org-a")
	ok3, _ := gate.TryAcquire("org-a")
	if !ok3 {
		t.Fatal("queued build must start after release")
	}
}

func TestOtherOrgsUnaffected(t *testing.T) {
	gate := NewBuildGate(BuildConcurrencyFree)
	if ok, _ := gate.TryAcquire("org-x"); !ok {
		t.Fatal("org-x must be admitted alongside org-y")
	}
}

func TestHighAdminCeilingPreserved(t *testing.T) {
	if AdminReqPerMinute <= TokenReqPerMinute {
		t.Fatal("admin ceiling must stay above the normal tier (must-not zero out)")
	}
}
