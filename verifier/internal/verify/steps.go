// Package verify implements the nine-step deployment verification chain
// (RB§27): every step must run — a running pod alone is never success.
package verify

import "time"

// StepTimeout caps every single step's execution.
const StepTimeout = 120 * time.Second

// Canonical step identifiers, ordered 1..9.
const (
	StepDeploymentAvailable = 1
	StepServiceReachable    = 2
	StepDNSResolve          = 3
	StepTLSHandshake        = 4
	StepRootHTTP            = 5
	StepAPIHealth           = 6
	StepBrowserLoad         = 7
	StepConsoleErrors       = 8
	StepFailedRequests      = 9
)

var stepNames = map[int]string{
	StepDeploymentAvailable: "deployment available",
	StepServiceReachable:    "service clusterip reachable",
	StepDNSResolve:          "dns resolves slug",
	StepTLSHandshake:        "tls handshake",
	StepRootHTTP:            "get / returns 2xx",
	StepAPIHealth:           "get /api/health returns 2xx",
	StepBrowserLoad:         "headless browser loads page",
	StepConsoleErrors:       "console error count is zero",
	StepFailedRequests:      "failed request count is zero",
}

var stepOrder = []int{
	StepDeploymentAvailable, StepServiceReachable, StepDNSResolve,
	StepTLSHandshake, StepRootHTTP, StepAPIHealth,
	StepBrowserLoad, StepConsoleErrors, StepFailedRequests,
}

// StepIDs returns a copy of the canonical ordered catalog.
func StepIDs() []int {
	return append([]int(nil), stepOrder...)
}

// StepName resolves a step's human label.
func StepName(id int) string { return stepNames[id] }

// IsBrowserStep reports whether the id belongs to the headless-browser trio
// that must never be skipped inside MVP verification.
func IsBrowserStep(id int) bool {
	return id >= StepBrowserLoad && id <= StepFailedRequests
}
