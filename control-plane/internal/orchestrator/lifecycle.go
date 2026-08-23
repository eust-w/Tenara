package orchestrator

import (
	"fmt"
	"time"
)

// RestartAnnotation patched onto workloads triggers a rolling restart while
// the orchestration state stays RUNNING.
const RestartAnnotation = "tenara.io/restarted-at"

// GuardRestart allows restarting only live applications.
func GuardRestart(state string) error {
	if state != StateRunning && state != StateDegraded {
		return fmt.Errorf("%w: restart requires %s|%s, got %s",
			ErrIllegalTransition, StateRunning, StateDegraded, state)
	}
	return nil
}

// GuardStop moves a live application to STOPPED through the R1 table.
func GuardStop(from string, at time.Time) (AuditEvent, error) {
	return Transition(from, StateStopped, ActorUser, at)
}

// GuardStart resumes a stopped application by re-entering the pipeline at
// PLANNED, where the next deploy rebuilds and re-verifies everything.
func GuardStart(from string, at time.Time) (AuditEvent, error) {
	return Transition(from, StatePlanned, ActorUser, at)
}

// ReplicasFor returns the desired replica count per orchestration state:
// STOPPED scales the workload to zero, every other state keeps capacity.
func ReplicasFor(state string, normal int32) int32 {
	if state == StateStopped {
		return 0
	}
	return normal
}

// SuspendedRoute records a removed HTTPRoute manifest while an app is
// stopped, so start can restore routing exactly and no hostname keeps
// resolving into an empty backend (RB§8 must-not).
type SuspendedRoute struct {
	SuspendedAt time.Time
	AppID       string
	Hostname    string
	RouteSpec   string
}

// SuspendRoute builds the restoration record for one removed route.
func SuspendRoute(appID, hostname, routeSpec string, at time.Time) SuspendedRoute {
	return SuspendedRoute{
		AppID:       appID,
		Hostname:    hostname,
		RouteSpec:   routeSpec,
		SuspendedAt: at,
	}
}
