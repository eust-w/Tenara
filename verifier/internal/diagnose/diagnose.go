// Package diagnose classifies deployment failures into known patterns and
// returns structured hints for the calling agent (RB§28 MVP scope: diagnose
// only — no auto-repair loops).
package diagnose

import (
	"strings"
	"time"

	"tenara/verifier/internal/verify"
)

// Known-pattern classification codes.
const (
	ClassMissingEnvDatabaseURL = "MISSING_ENV_DATABASE_URL"
	ClassCrashLoopBackOff      = "CRASH_LOOP_BACK_OFF"
	ClassOOMKilled             = "OOM_KILLED"
	ClassImagePullBackOff      = "IMAGE_PULL_BACK_OFF"
	ClassUpstream502           = "UPSTREAM_502"
	ClassUnclassified          = "UNCLASSIFIED"
)

// Bundle aggregates every evidence source available after a failed deploy.
// Raw payloads are consumed here and never copied outward.
type Bundle struct {
	GeneratedAt   time.Time
	VerifyReport  *verify.Report
	BuildLogTail  string
	PodEvents     string
	ContainerLogs string
	AppID         string
}

// Diagnosis is the structured result handed back through app.status and
// GET /diagnostics. It intentionally carries only codes, hints and step ids —
// embedding raw log text would risk leaking secrets.
type Diagnosis struct {
	GeneratedAt time.Time
	FailedSteps []int
	AppID       string
	Classified  string
	Hint        string
}

type patternRule struct {
	class    string
	hint     string
	keywords []string
}

// rules is the known-pattern table; first match wins.
var rules = []patternRule{
	{
		class:    ClassMissingEnvDatabaseURL,
		hint:     "bind a mongo database so DATABASE_URL can be injected",
		keywords: []string{"DATABASE_URL"},
	},
	{
		class:    ClassCrashLoopBackOff,
		hint:     "inspect container logs for a repeated startup crash",
		keywords: []string{"CrashLoopBackOff"},
	},
	{
		class:    ClassOOMKilled,
		hint:     "trim memory usage or raise the plan quota",
		keywords: []string{"OOMKilled", "out of memory"},
	},
	{
		class:    ClassImagePullBackOff,
		hint:     "check the image digest exists in the registry",
		keywords: []string{"ImagePullBackOff", "ErrImagePull"},
	},
	{
		class:    ClassUpstream502,
		hint:     "backend is not answering on its declared port",
		keywords: []string{"502 Bad Gateway"},
	},
}

// Classify scans the bundle against the known-pattern table and collects the
// failed verify steps; unmatched failures stay UNCLASSIFIED.
func Classify(b Bundle) Diagnosis {
	d := Diagnosis{
		GeneratedAt: b.GeneratedAt,
		AppID:       b.AppID,
		Classified:  ClassUnclassified,
	}
	if b.VerifyReport != nil {
		for _, s := range b.VerifyReport.Steps {
			if s.Status == verify.StatusFail {
				d.FailedSteps = append(d.FailedSteps, s.ID)
			}
		}
	}

	haystack := strings.Join([]string{
		b.BuildLogTail, b.PodEvents, b.ContainerLogs,
	}, "\n")
	for _, rule := range rules {
		for _, kw := range rule.keywords {
			if strings.Contains(haystack, kw) {
				d.Classified = rule.class
				d.Hint = rule.hint
				return d
			}
		}
	}
	return d
}
