package build

import (
	"strings"
	"testing"
)

func TestScanReportObjectKey(t *testing.T) {
	if got := ScanReportObjectKey("b1"); got != "artifacts/b1/report.json" {
		t.Fatalf("key = %q", got)
	}
}

func TestTrivyScanArgs(t *testing.T) {
	ref := ScanRef(ImageTag("app-1", "abcdef1234567890"), "sha256:abc")
	args := TrivyScanArgs(ref)
	joined := strings.Join(args, "\n")

	for _, want := range []string{
		"image",
		"--severity", "CRITICAL",
		"--exit-code", "1",
		scanReportOutputPath,
		ref,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("trivy args missing %q", want)
		}
	}
	if args[len(args)-1] != ref {
		t.Fatalf("image ref must be the final arg, got %q", args[len(args)-1])
	}
}

func TestEvaluateScanGate(t *testing.T) {
	b := sampleBuild()
	b.Status.Phase = PhaseScanning

	if err := EvaluateScanGate(b, 0); err != nil {
		t.Fatalf("clean scan must pass: %v", err)
	}
	if b.Status.Phase == PhaseFailed {
		t.Fatal("clean scan must not fail the build")
	}

	if err := EvaluateScanGate(b, 3); err == nil {
		t.Fatal("CRITICAL findings must fail the gate")
	}
	if b.Status.Phase != PhaseFailed || b.Status.Reason != scanFailedReason {
		t.Fatalf("status = %+v", b.Status)
	}
}
