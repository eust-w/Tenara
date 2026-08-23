package orchestrator

import (
	"errors"
	"testing"
	"time"
)

func TestHappyPathChainIsLegal(t *testing.T) {
	chain := []string{
		StatePlanned, StateBuilding, StateDeploying,
		StateVerifying, StateRunning,
	}
	for i := range len(chain) - 1 {
		if !CanTransition(chain[i], chain[i+1]) {
			t.Fatalf("%s -> %s must be legal", chain[i], chain[i+1])
		}
	}
}

func TestSkipAndForeignStatesAreIllegal(t *testing.T) {
	for _, pair := range [][2]string{
		{StateBuilding, StateRunning},
		{StatePlanned, StateRunning},
		{"CREATED", StateBuilding},
		{"CREATED", StateRunning},
		{StateDeleted, StateBuilding},
	} {
		if CanTransition(pair[0], pair[1]) {
			t.Fatalf("%s -> %s must be illegal", pair[0], pair[1])
		}
		if _, err := Transition(pair[0], pair[1], ActorController, time.Now()); !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("%s -> %s want ErrIllegalTransition, got %v", pair[0], pair[1], err)
		}
	}
}

func TestTransitionReturnsAuditPayload(t *testing.T) {
	at := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ev, err := Transition(StateVerifying, StateRunning, ActorController, at)
	if err != nil {
		t.Fatal(err)
	}
	if ev.FromState != StateVerifying || ev.ToState != StateRunning ||
		ev.ActorType != ActorController || !ev.At.Equal(at) {
		t.Fatalf("audit event = %+v", ev)
	}
}

func TestRollbackStopDeleteBranches(t *testing.T) {
	legal := [][2]string{
		{StateRunning, StateRollingBack},
		{StateRollingBack, StateVerifying},
		{StateRunning, StateStopped},
		{StateStopped, StatePlanned},
		{StateRunning, StateDeleting},
		{StateDeleting, StateDeleted},
		{StateDegraded, StateRollingBack},
		{StateDegraded, StateStopped},
		{StateFailed, StatePlanned},
	}
	for _, pair := range legal {
		if !CanTransition(pair[0], pair[1]) {
			t.Fatalf("%s -> %s must be legal", pair[0], pair[1])
		}
	}
}

func TestDeployLocksConflictThenRelease(t *testing.T) {
	locks := NewDeployLocks()
	if err := locks.Acquire("app-1"); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if err := locks.Acquire("app-1"); !errors.Is(err, ErrConcurrentDeploy) {
		t.Fatalf("second acquire want ErrConcurrentDeploy, got %v", err)
	}
	if err := locks.Acquire("app-2"); err != nil {
		t.Fatalf("other apps unaffected: %v", err)
	}
	locks.Release("app-1")
	if err := locks.Acquire("app-1"); err != nil {
		t.Fatalf("release must free the slot: %v", err)
	}
}
