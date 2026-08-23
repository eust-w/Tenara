package build

import (
	"strings"
	"testing"
)

func TestSBOMObjectKey(t *testing.T) {
	if got := SBOMObjectKey("b1"); got != "artifacts/b1/sbom.json" {
		t.Fatalf("key = %q", got)
	}
}

func TestScanRef(t *testing.T) {
	tag := ImageTag("app-1", "abcdef1234567890")
	got := ScanRef(tag, "sha256:abc")
	if got != tag+"@sha256:abc" {
		t.Fatalf("ref = %q", got)
	}
	if ScanRef("", "sha256:abc") != "" || ScanRef(tag, "") != "" {
		t.Fatal("empty components must yield empty ref")
	}
}

func TestSyftScanArgs(t *testing.T) {
	args := SyftScanArgs(ScanRef(ImageTag("app-1", "abcdef1234567890"), "sha256:abc"))
	joined := strings.Join(args, "\n")

	for _, want := range []string{"scan", "-o", "spdx-json=" + sbomOutputPath} {
		if !strings.Contains(joined, want) {
			t.Fatalf("syft args missing %q", want)
		}
	}
}

func TestRequireSBOMRefEnforcement(t *testing.T) {
	b := sampleBuild()
	b.Status.Phase = PhaseScanning

	if err := RequireSBOMRef(b); err == nil {
		t.Fatal("missing SBOM must fail")
	}
	if b.Status.Phase != PhaseFailed || b.Status.Reason != "sbom-missing" {
		t.Fatalf("missing SBOM must not allow success: %+v", b.Status)
	}

	SetSBOMRef(b, SBOMObjectKey("b1"))
	if err := RequireSBOMRef(b); err != nil {
		t.Fatalf("with SBOM ref must pass: %v", err)
	}
}
