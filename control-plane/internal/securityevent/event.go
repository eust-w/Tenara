// Package securityevent defines the unified security_events schema and event
// code catalog (RB§31 §32). Rows are admin-only readable — ordinary users
// never receive an endpoint to this data.
package securityevent

import (
	"errors"
	"fmt"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityCritical Severity = "critical"
)

// Event codes emitted across the platform write points.
const (
	CodeLoginFailed            = "auth.login.failed"
	CodeUserSuspended          = "user.suspended"
	CodeBuildConcurrencyDenied = "build.concurrency.denied"
	CodeRateLimitRejected      = "rate.limit.rejected"
	CodeNetpolDenied           = "netpol.denied"
)

var catalog = map[string]struct {
	severity Severity
}{
	CodeLoginFailed:            {severity: SeverityWarn},
	CodeUserSuspended:          {severity: SeverityWarn},
	CodeBuildConcurrencyDenied: {severity: SeverityInfo},
	CodeRateLimitRejected:      {severity: SeverityInfo},
	CodeNetpolDenied:           {severity: SeverityWarn},
}

type Event struct {
	EventCode string
	Severity  Severity
	ActorType string
	ActorID   string
	OrgID     string
	AppID     string
	Detail    string
	At        time.Time
}

// Validate enforces the unified schema before persistence.
func Validate(e Event) error {
	severity, known := catalog[e.EventCode]
	if !known {
		return fmt.Errorf("unknown event code %q", e.EventCode)
	}
	if e.At.IsZero() {
		return errors.New("event timestamp is required")
	}
	if e.ActorType != "user" && e.ActorType != "controller" {
		return fmt.Errorf("invalid actor_type %q", e.ActorType)
	}
	if e.Severity == "" {
		e.Severity = severity.severity
	}
	return nil
}
