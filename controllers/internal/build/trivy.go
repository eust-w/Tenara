package build

import (
	"errors"
	"fmt"
)

const (
	trivyImage           = "aquasec/trivy:0.58.2"
	scanReportOutputPath = "/out/report.json"
	reportSuffix         = "report.json"
	scanFailedReason     = "scan-failed"
)

// ScanReportObjectKey returns the MinIO object key storing a build's Trivy
// scan report.
func ScanReportObjectKey(buildID string) string {
	return fmt.Sprintf("artifacts/%s/%s", buildID, reportSuffix)
}

// TrivyScanArgs returns the trivy CLI arguments for the vulnerability gate.
// Per §53.1 trivy exits 0 on findings by default, so --exit-code 1 must be
// explicit; there is deliberately no flag to disable the severity gate.
func TrivyScanArgs(scanRef string) []string {
	return []string{
		"image",
		"--severity", "CRITICAL",
		"--exit-code", "1",
		"--output", scanReportOutputPath,
		scanRef,
	}
}

// EvaluateScanGate fails the build when any CRITICAL finding exists, blocking
// all subsequent signing/pushing stages.
func EvaluateScanGate(b *Build, criticalCount int) error {
	if criticalCount > 0 {
		MarkFailed(b, scanFailedReason,
			fmt.Sprintf("%d CRITICAL vulnerabilities; sign/push blocked", criticalCount))
		return errors.New(scanFailedReason)
	}
	return nil
}
