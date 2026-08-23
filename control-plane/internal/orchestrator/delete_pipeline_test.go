package orchestrator

import (
	"errors"
	"testing"
	"time"
)

func TestDeleteCatalogSevenOrdered(t *testing.T) {
	ids := DeleteStepIDs()
	if len(ids) != 7 {
		t.Fatalf("catalog size = %d, want 7", len(ids))
	}
	for i, id := range ids {
		if id != i+1 {
			t.Fatalf("catalog[%d] = %d", i, id)
		}
		if DeleteStepName(id) == "" {
			t.Fatalf("step %d lacks name", id)
		}
	}
}

func TestNextDeleteStepAdvancesStrictly(t *testing.T) {
	cases := []struct {
		completed []int
		want      int
	}{
		{completed: nil, want: DelStepTrafficDisabled},
		{completed: []int{DelStepTrafficDisabled}, want: DelStepWorkloadStopped},
		{completed: []int{1, 2, 3, 4, 5, 6}, want: DelStepDataPurged},
	}
	for _, tc := range cases {
		got, err := NextDeleteStep(tc.completed)
		if err != nil || got != tc.want {
			t.Fatalf("completed=%v got (%d,%v)", tc.completed, got, err)
		}
	}
	if _, err := NextDeleteStep([]int{1, 2, 3, 4, 5, 6, 7}); !errors.Is(err, ErrPipelineComplete) {
		t.Fatalf("full pipeline want ErrPipelineComplete, got %v", err)
	}
	if _, err := NextDeleteStep([]int{9}); err == nil {
		t.Fatal("unknown completed id must fail closed")
	}
}

func TestRetentionDefaultsAndAcceleratedClock(t *testing.T) {
	t.Setenv(retentionHoursEnv, "")
	d, err := RetentionPeriod()
	if err != nil || d != DefaultRetentionDays*24*time.Hour {
		t.Fatalf("default retention = %s", d)
	}

	t.Setenv(retentionHoursEnv, "0.01")
	d, err = RetentionPeriod()
	if err != nil || d.String() != "36s" {
		t.Fatalf("accelerated retention = %s, want 36s", d)
	}

	t.Setenv(retentionHoursEnv, "-3")
	if _, err = RetentionPeriod(); err == nil {
		t.Fatal("negative retention must be rejected")
	}
	t.Setenv(retentionHoursEnv, "abc")
	if _, err = RetentionPeriod(); err == nil {
		t.Fatal("garbage retention must be rejected")
	}
}

func TestPurgeGatedBehindRetentionWindow(t *testing.T) {
	deletedAt := time.Unix(1700000000, 0)
	until := SoftDeleteUntil(deletedAt, 7*24*time.Hour)
	if !PurgeAllowed(until.Add(time.Second), until) {
		t.Fatal("purge after window must be allowed")
	}
	if PurgeAllowed(deletedAt, until) {
		t.Fatal("purge inside the window must be blocked")
	}
}

func TestRestoreWindowClosesAtSoftDelete(t *testing.T) {
	open := [][]int{nil, {1}, {1, 2}, {1, 2, 3}}
	for _, completed := range open {
		if !RestoreWindowOpen(completed) {
			t.Fatalf("window must stay open at %v", completed)
		}
	}
	closed := [][]int{{4}, {1, 2, 3, 4}, {1, 2, 3, 4, 5, 6, 7}}
	for _, completed := range closed {
		if RestoreWindowOpen(completed) {
			t.Fatalf("window must be closed at %v", completed)
		}
	}
}

func TestRestoreEscapesThroughTransitionTable(t *testing.T) {
	at := time.Unix(0, 0)
	ev, err := Transition(StateDeleting, StatePlanned, ActorUser, at)
	if err != nil {
		t.Fatalf("mid-pipeline restore must escape via PLANNED: %v", err)
	}
	if ev.ToState != StatePlanned {
		t.Fatalf("to = %s", ev.ToState)
	}
	if CanTransition(StateDeleted, StatePlanned) {
		t.Fatal("terminal DELETED must have no escape edge")
	}
}
