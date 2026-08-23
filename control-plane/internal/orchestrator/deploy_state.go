package orchestrator

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Deployment lifecycle states (RB§26). CREATED lives only inside revision
// rows before approval and is intentionally absent from the transition table.
const (
	StatePlanned     = "PLANNED"
	StateBuilding    = "BUILDING"
	StateDeploying   = "DEPLOYING"
	StateVerifying   = "VERIFYING"
	StateRunning     = "RUNNING"
	StateDegraded    = "DEGRADED"
	StateFailed      = "FAILED"
	StateRollingBack = "ROLLING_BACK"
	StateStopped     = "STOPPED"
	StateDeleting    = "DELETING"
	StateDeleted     = "DELETED"
)

// ErrIllegalTransition guards out-of-table moves (R1).
var ErrIllegalTransition = errors.New("illegal deployment transition")

// ErrConcurrentDeploy carries 409 semantics when a second deploy targets an
// app that already has one in flight.
var ErrConcurrentDeploy = errors.New("another deployment is already in flight")

// allowedTransitions is the table-driven guard: states missing from the map
// reject every outgoing move.
var allowedTransitions = map[string]map[string]bool{
	StatePlanned:   {StateBuilding: true},
	StateBuilding:  {StateDeploying: true, StateFailed: true},
	StateDeploying: {StateVerifying: true, StateFailed: true},
	StateVerifying: {StateRunning: true, StateDegraded: true, StateFailed: true},
	StateRunning: {
		StateVerifying: true, StateDegraded: true,
		StateRollingBack: true, StateStopped: true, StateDeleting: true,
	},
	StateDegraded:    {StateVerifying: true, StateRollingBack: true, StateFailed: true, StateDeleting: true},
	StateFailed:      {StatePlanned: true, StateDeleting: true},
	StateRollingBack: {StateVerifying: true, StateFailed: true},
	StateStopped:     {StatePlanned: true, StateDeleting: true},
	StateDeleting:    {StateDeleted: true},
}

// CanTransition reports whether the move is table-approved.
func CanTransition(from, to string) bool {
	return allowedTransitions[from][to]
}

// AuditEvent is the persisted trail produced by every accepted transition;
// controller-driven moves always use ActorController.
type AuditEvent struct {
	At        time.Time `json:"at"`
	AppID     string    `json:"app_id"`
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	ActorType string    `json:"actor_type"`
}

// Actor types writing orchestration audit rows.
const (
	ActorController = "controller"
	ActorUser       = "user"
)

// Transition validates the move table-side and returns its audit payload.
func Transition(from, to, actorType string, at time.Time) (AuditEvent, error) {
	if !CanTransition(from, to) {
		return AuditEvent{}, fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, from, to)
	}
	return AuditEvent{FromState: from, ToState: to, ActorType: actorType, At: at}, nil
}

// DeployLocks serialises in-flight deploys per app so a concurrent second
// deploy surfaces as ErrConcurrentDeploy instead of racing.
type DeployLocks struct {
	inFlight map[string]bool
	mu       sync.Mutex
}

// NewDeployLocks builds an empty lock registry.
func NewDeployLocks() *DeployLocks {
	return &DeployLocks{inFlight: map[string]bool{}}
}

// Acquire marks an app as deploying; a second acquire conflicts.
func (l *DeployLocks) Acquire(appID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight[appID] {
		return fmt.Errorf("%w: app %s", ErrConcurrentDeploy, appID)
	}
	l.inFlight[appID] = true
	return nil
}

// Release frees the app slot after the deploy settles.
func (l *DeployLocks) Release(appID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inFlight, appID)
}
