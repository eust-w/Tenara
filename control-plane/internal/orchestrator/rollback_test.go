package orchestrator

import (
	"errors"
	"testing"
	"time"
)

func snap(n int, scanPassed, signed bool) RevisionSnapshot {
	s := RevisionSnapshot{
		Number:      n,
		ImageDigest: "sha256:testdigest",
		ScanPassed:  scanPassed,
		ConfigRev:   n, SecretRev: n, AppSpecVer: n,
	}
	if signed {
		s.SignatureRef = "artifacts/rev-" + time.Duration(n).String() + "/sig"
	}
	return s
}

func sampleHistory() []RevisionSnapshot {
	return []RevisionSnapshot{
		snap(1, true, true),
		snap(2, false, false),
		snap(3, true, false),
		snap(4, true, true),
	}
}

func TestDefaultTargetsNewestEligiblePredecessor(t *testing.T) {
	dest, err := ResolveRollbackTarget(sampleHistory(), 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dest.Number != 4 {
		t.Fatalf("default dest = %d, want 4", dest.Number)
	}
}

func TestDefaultSkipsUnscannedAndUnsigned(t *testing.T) {
	dest, err := ResolveRollbackTarget(sampleHistory(), 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dest.Number != 1 {
		t.Fatalf("skipping rev3(unsigned)/rev2(unscanned) should land on 1, got %d", dest.Number)
	}
}

func TestExplicitMissingOrFutureTargetIsNotFound(t *testing.T) {
	for _, bad := range []int{99, 0, -1} {
		n := bad
		if _, err := ResolveRollbackTarget(sampleHistory(), 5, &n); !errors.Is(err, ErrRevisionNotFound) {
			t.Fatalf("target %d want ErrRevisionNotFound, got %v", bad, err)
		}
	}
	future := 5
	if _, err := ResolveRollbackTarget(sampleHistory(), 5, &future); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatal("rolling forward must be treated as not-found")
	}
}

func TestExplicitUnscannedOrUnsignedTargetRejected(t *testing.T) {
	two, three := 2, 3
	if _, err := ResolveRollbackTarget(sampleHistory(), 5, &two); !errors.Is(err, ErrScanNotPassed) {
		t.Fatalf("unscanned target want ErrScanNotPassed, got %v", err)
	}
	if _, err := ResolveRollbackTarget(sampleHistory(), 5, &three); !errors.Is(err, ErrScanNotPassed) {
		t.Fatalf("unsigned target want ErrScanNotPassed, got %v", err)
	}
}

func TestNoEligiblePathWhenGatesNeverCleared(t *testing.T) {
	history := []RevisionSnapshot{snap(1, false, false), snap(2, true, false)}
	if _, err := ResolveRollbackTarget(history, 2, nil); !errors.Is(err, ErrNoRollbackPath) {
		t.Fatalf("want ErrNoRollbackPath, got %v", err)
	}
}

func TestPlanRollbackCombinesGuardAndTarget(t *testing.T) {
	at := time.Unix(1700000000, 0)

	ev, dest, err := PlanRollback(StateRunning, sampleHistory(), 5, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if ev.FromState != StateRunning || ev.ToState != StateRollingBack ||
		ev.ActorType != ActorUser || !ev.At.Equal(at) {
		t.Fatalf("audit = %+v", ev)
	}
	if dest.Number != 4 {
		t.Fatalf("dest = %d", dest.Number)
	}

	if _, _, guardErr := PlanRollback(StateBuilding, sampleHistory(), 5, nil, at); !errors.Is(guardErr, ErrIllegalTransition) {
		t.Fatalf("state guard must fire before any target lookup, got %v", guardErr)
	}

	if _, _, targetErr := PlanRollback(StateRunning, sampleHistory(), 5, new(int), at); !errors.Is(targetErr, ErrRevisionNotFound) {
		t.Fatalf("target errors must pass through after the guard, got %v", targetErr)
	}
}
