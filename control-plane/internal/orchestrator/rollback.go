package orchestrator

import (
	"errors"
	"fmt"
	"time"
)

// RevisionSnapshot is the minimal per-revision view needed to authorise a
// rollback (RB§26 snapshot semantics).
type RevisionSnapshot struct {
	ImageDigest  string
	SignatureRef string
	Number       int
	ConfigRev    int
	SecretRev    int
	AppSpecVer   int
	ScanPassed   bool
}

var (
	// ErrRevisionNotFound carries 404 semantics.
	ErrRevisionNotFound = errors.New("revision not found")
	// ErrScanNotPassed carries 400 semantics: the destination never cleared
	// the trivy/cosign gates, so rolling back to it is forbidden.
	ErrScanNotPassed = errors.New("target revision did not pass scan/signature gates")
	// ErrNoRollbackPath means no eligible predecessor exists at all.
	ErrNoRollbackPath = errors.New("no eligible rollback target")
)

func eligible(s RevisionSnapshot) bool {
	return s.ScanPassed && s.SignatureRef != ""
}

func findSnapshot(revs []RevisionSnapshot, number int) (RevisionSnapshot, bool) {
	for _, s := range revs {
		if s.Number == number {
			return s, true
		}
	}
	return RevisionSnapshot{}, false
}

// ResolveRollbackTarget picks the destination revision. An explicit target
// must exist behind current and must have cleared both gates; the default
// picks the newest eligible predecessor, skipping unscanned ones entirely.
func ResolveRollbackTarget(revs []RevisionSnapshot, current int, target *int) (RevisionSnapshot, error) {
	if target != nil {
		s, ok := findSnapshot(revs, *target)
		if !ok || s.Number >= current {
			return RevisionSnapshot{}, fmt.Errorf("%w: %d", ErrRevisionNotFound, *target)
		}
		if !eligible(s) {
			return RevisionSnapshot{}, fmt.Errorf("%w: %d", ErrScanNotPassed, *target)
		}
		return s, nil
	}

	best := RevisionSnapshot{}
	found := false
	for _, s := range revs {
		if s.Number >= current || !eligible(s) {
			continue
		}
		if !found || s.Number > best.Number {
			best, found = s, true
		}
	}
	if !found {
		return RevisionSnapshot{}, fmt.Errorf("%w: nothing behind %d cleared the gates",
			ErrNoRollbackPath, current)
	}
	return best, nil
}

// PlanRollback combines the R1 state move into ROLLING_BACK with target
// resolution, returning the audit trail and the destination snapshot.
func PlanRollback(fromState string, revs []RevisionSnapshot, current int,
	target *int, at time.Time,
) (AuditEvent, RevisionSnapshot, error) {
	ev, tErr := Transition(fromState, StateRollingBack, ActorUser, at)
	if tErr != nil {
		return AuditEvent{}, RevisionSnapshot{}, tErr
	}
	dest, rErr := ResolveRollbackTarget(revs, current, target)
	if rErr != nil {
		return AuditEvent{}, RevisionSnapshot{}, rErr
	}
	return ev, dest, nil
}
