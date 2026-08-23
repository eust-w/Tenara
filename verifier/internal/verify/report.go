package verify

import (
	"fmt"
	"time"
)

// Status is a single step outcome.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
)

// Outcomes written back into orchestration after aggregation.
const (
	OutcomeRunning  = "RUNNING"
	OutcomeDegraded = "DEGRADED"
)

// StepResult carries one step's verdict for the report.
type StepResult struct {
	Name       string `json:"name"`
	Status     Status `json:"status"`
	Detail     string `json:"detail,omitempty"`
	ID         int    `json:"id"`
	DurationMS int64  `json:"duration_ms"`
}

// Report is the artifact persisted for every verification run; keys are the
// stable lowercase contract stored under artifacts/.
type Report struct {
	GeneratedAt time.Time    `json:"generated_at"`
	AppID       string       `json:"app_id"`
	Steps       []StepResult `json:"steps"`
	Outcome     string       `json:"outcome"`
}

// Aggregate rolls step results into a report: any failed step degrades the
// whole application, otherwise it lands RUNNING.
func Aggregate(appID string, steps []StepResult, at time.Time) Report {
	outcome := OutcomeRunning
	for _, s := range steps {
		if s.Status == StatusFail {
			outcome = OutcomeDegraded
			break
		}
	}
	return Report{AppID: appID, Steps: steps, Outcome: outcome, GeneratedAt: at}
}

// ValidateComplete enforces the full ordered nine-step catalog: nothing may
// be dropped, reordered or renamed — browser steps included.
func ValidateComplete(steps []StepResult) error {
	if len(steps) != len(stepOrder) {
		return fmt.Errorf("expected %d steps, got %d", len(stepOrder), len(steps))
	}
	for i, s := range steps {
		if s.ID != i+1 {
			return fmt.Errorf("step position %d holds id %d", i+1, s.ID)
		}
		if s.Name != StepName(s.ID) {
			return fmt.Errorf("step %d name %q does not match catalog", s.ID, s.Name)
		}
	}
	return nil
}
