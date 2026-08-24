// Package repair implements the in-platform auto-repair loop (RB§28 full
// form, D2-P2-5 / todo92): verify failures feed the todo56 diagnostics
// bundle to an LLM patch source, whose remediation rebuilds/redeploys and
// re-verifies under a hard max-attempts gate with per-attempt auditing.
package repair

import (
	"context"
	"errors"
	"fmt"
)

// MaxAttempts is the hard gate from the design doc: attempts one through
// three may fire; triggering a fourth is refused outright.
const MaxAttempts = 3

// ErrAttemptsExhausted marks refusal of further auto-repair triggers.
var ErrAttemptsExhausted = errors.New("auto-repair attempts exhausted")

// CanTrigger reports whether firing attempt n is permitted; the fourth call
// must never reach a patch source.
func CanTrigger(n int) error {
	if n < 1 {
		return fmt.Errorf("attempt number %d out of range", n)
	}
	if n > MaxAttempts {
		return fmt.Errorf("%w: attempt %d exceeds cap %d", ErrAttemptsExhausted, n, MaxAttempts)
	}
	return nil
}

// DiagnosticsBundle is the todo56 classifier payload handed to the patch
// source (e.g. missing env wiring like DATABASE_URL).
type DiagnosticsBundle struct {
	AppID   string
	Summary string   // classifier verdict
	Events  []string // ordered evidence lines
}

// Patch is a proposed remediation produced by an LLM source.
type Patch struct {
	Description string
	Files       map[string][]byte // path -> new content
}

// PatchSource abstracts Codex/Claude-style remediation generators; live
// implementations are gated by API credentials (design-doc dependency).
type PatchSource interface {
	GeneratePatch(ctx context.Context, b DiagnosticsBundle) (Patch, error)
}

// Auditor records every attempt for RB§28 audit completeness; failures of
// recording surface to the caller instead of being swallowed.
type Auditor interface {
	RecordAttempt(ctx context.Context, appID string, n int, p Patch, attemptErr error) error
}

// ApplyVerify applies a patch and re-runs the verify chain until RUNNING;
// injected so the loop stays testable without clusters or builders.
type ApplyVerify func(ctx context.Context, p Patch) error

// Loop drives diagnose -> patch -> apply+verify under the hard gate.
type Loop struct {
	Source  PatchSource
	Auditor Auditor
	Apply   ApplyVerify
}

// Run repairs appID from bundle or returns the terminal failure: either the
// attempts-exhausted refusal (fourth trigger never fires) or the last
// underlying error joined across generate/apply stages.
func (l *Loop) Run(ctx context.Context, appID string, b DiagnosticsBundle) error {
	var lastErr error
	for n := 1; ; n++ {
		if gateErr := CanTrigger(n); gateErr != nil {
			if l.Auditor != nil {
				if recErr := l.Auditor.RecordAttempt(ctx, appID, n, Patch{}, gateErr); recErr != nil {
					return recErr
				}
			}
			return gateErr
		}
		patch, genErr := l.Source.GeneratePatch(ctx, b)
		var verifyErr error
		if genErr == nil && l.Apply != nil {
			verifyErr = l.Apply(ctx, patch)
		}
		lastErr = errors.Join(genErr, verifyErr)
		if l.Auditor != nil {
			if recErr := l.Auditor.RecordAttempt(ctx, appID, n, patch, lastErr); recErr != nil {
				return recErr
			}
		}
		if lastErr == nil {
			return nil // repaired: verify chain back to RUNNING
		}
	}
}
