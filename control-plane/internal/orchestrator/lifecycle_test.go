package orchestrator

import (
	"errors"
	"testing"
	"time"
)

func TestRestartOnlyFromLiveStates(t *testing.T) {
	for _, live := range []string{StateRunning, StateDegraded} {
		if err := GuardRestart(live); err != nil {
			t.Fatalf("restart from %s must pass: %v", live, err)
		}
	}
	for _, dead := range []string{StateBuilding, StateStopped, StatePlanned} {
		if err := GuardRestart(dead); !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("restart from %s want ErrIllegalTransition, got %v", dead, err)
		}
	}
}

func TestStopAndStartWalkTheTable(t *testing.T) {
	at := time.Unix(1700000000, 0)

	ev, sErr := GuardStop(StateRunning, at)
	if sErr != nil || ev.ToState != StateStopped || ev.ActorType != ActorUser {
		t.Fatalf("stop audit = %+v err=%v", ev, sErr)
	}
	if _, dErr := GuardStop(StatePlanned, at); !errors.Is(dErr, ErrIllegalTransition) {
		t.Fatalf("stop from planned must be illegal, got %v", dErr)
	}

	startEv, gErr := GuardStart(StateStopped, at)
	if gErr != nil || startEv.ToState != StatePlanned {
		t.Fatalf("start audit = %+v err=%v", startEv, gErr)
	}
	if _, bad := GuardStart(StateRunning, at); !errors.Is(bad, ErrIllegalTransition) {
		t.Fatal("start from running must be illegal")
	}
}

func TestReplicasScaleToZeroOnlyWhenStopped(t *testing.T) {
	if got := ReplicasFor(StateStopped, 2); got != 0 {
		t.Fatalf("stopped replicas = %d, want 0", got)
	}
	for _, state := range []string{StateRunning, StateDegraded, StateVerifying} {
		if got := ReplicasFor(state, 2); got != 2 {
			t.Fatalf("%s replicas = %d, want 2", state, got)
		}
	}
}

func TestSuspendRouteRoundTrip(t *testing.T) {
	at := time.Unix(1700000000, 0)
	rec := SuspendRoute("acme", "acme.127.0.0.1.nip.io", "kind: HTTPRoute\n...", at)
	if rec.AppID != "acme" || rec.Hostname == "" || rec.RouteSpec == "" || !rec.SuspendedAt.Equal(at) {
		t.Fatalf("record = %+v", rec)
	}
}
