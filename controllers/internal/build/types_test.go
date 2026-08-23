package build

import (
	"testing"
)

func TestPhaseConstants(t *testing.T) {
	if PhaseCreated != "CREATED" || PhaseCloning != "CLONING" ||
		PhaseBuilding != "BUILDING" || PhaseScanning != "SCANNING" ||
		PhaseSigning != "SIGNING" || PhasePushed != "PUSHED" ||
		PhaseFailed != "FAILED" {
		t.Fatal("phase constants mismatch RB-26 lifecycle")
	}
}

func TestSchemeHasBuilder(t *testing.T) {
	if Scheme == nil {
		t.Fatal("Scheme must not be nil")
	}
}

func TestBuildSpecFields(t *testing.T) {
	s := BuildSpec{
		AppID:      "app-123",
		Env:        "production",
		Git:        GitSource{URL: "https://github.com/x/y", SHA: "abc"},
		AppSpecRef: "spec-ref",
		Dockerfile: "./Dockerfile",
	}
	if s.AppID != "app-123" || s.Env != "production" || s.Git.URL != "https://github.com/x/y" {
		t.Fatalf("spec fields wrong: %+v", s)
	}
}
