package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Ordered soft-delete pipeline steps (RB§35). Every transition persists its
// completion before the next one runs; nothing ever drops data in one shot.
const (
	DelStepTrafficDisabled     = 1
	DelStepWorkloadStopped     = 2
	DelStepDBSnapshotted       = 3
	DelStepSoftDeleted         = 4
	DelStepRuntimeDestroyed    = 5
	DelStepCredentialDestroyed = 6
	DelStepDataPurged          = 7

	DefaultRetentionDays = 7
	retentionHoursEnv    = "TENARA_RETENTION_HOURS"
)

var deleteStepNames = map[int]string{
	DelStepTrafficDisabled:     "traffic disabled",
	DelStepWorkloadStopped:     "workload stopped",
	DelStepDBSnapshotted:       "database snapshotted to backups/",
	DelStepSoftDeleted:         "soft deleted (retention window)",
	DelStepRuntimeDestroyed:    "runtime namespace destroyed",
	DelStepCredentialDestroyed: "credentials destroyed",
	DelStepDataPurged:          "backups and data purged",
}

var deleteStepOrder = []int{
	DelStepTrafficDisabled, DelStepWorkloadStopped, DelStepDBSnapshotted,
	DelStepSoftDeleted, DelStepRuntimeDestroyed, DelStepCredentialDestroyed,
	DelStepDataPurged,
}

// ErrPipelineComplete marks a fully drained deletion pipeline.
var ErrPipelineComplete = errors.New("deletion pipeline already complete")

// DeleteStepIDs returns the canonical ordered catalog.
func DeleteStepIDs() []int {
	return append([]int(nil), deleteStepOrder...)
}

// DeleteStepName resolves a step's human label.
func DeleteStepName(id int) string { return deleteStepNames[id] }

// NextDeleteStep returns the first unfinished step; completing all seven
// yields ErrPipelineComplete. Unknown completed ids fail closed.
func NextDeleteStep(completed []int) (int, error) {
	done := make(map[int]bool, len(completed))
	for _, c := range completed {
		if c < DelStepTrafficDisabled || c > DelStepDataPurged {
			return 0, fmt.Errorf("unknown deletion step %d", c)
		}
		done[c] = true
	}
	for _, id := range deleteStepOrder {
		if !done[id] {
			return id, nil
		}
	}
	return 0, ErrPipelineComplete
}

// RetentionPeriod reads TENARA_RETENTION_HOURS (fractional hours allowed so
// tests can accelerate the clock); it defaults to seven days.
func RetentionPeriod() (time.Duration, error) {
	value := os.Getenv(retentionHoursEnv)
	if value == "" {
		return DefaultRetentionDays * 24 * time.Hour, nil
	}
	hours, err := strconv.ParseFloat(value, 64)
	if err != nil || hours < 0 {
		return 0, fmt.Errorf("%s must be a non-negative number of hours, got %q",
			retentionHoursEnv, value)
	}
	return time.Duration(hours * float64(time.Hour)), nil
}

// SoftDeleteUntil stamps when physical purging becomes permitted.
func SoftDeleteUntil(deletedAt time.Time, retention time.Duration) time.Time {
	return deletedAt.Add(retention)
}

// PurgeAllowed gates the final destructive step behind the retention window.
func PurgeAllowed(now, retentionUntil time.Time) bool {
	return !now.Before(retentionUntil)
}

// RestoreWindowOpen reports whether mid-pipeline restore is still possible:
// once the app is marked soft_deleted (step 4) or beyond, the answer is no.
func RestoreWindowOpen(completed []int) bool {
	for _, c := range completed {
		if c >= DelStepSoftDeleted {
			return false
		}
	}
	return true
}
