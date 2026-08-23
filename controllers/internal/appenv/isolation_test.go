package appenv

import (
	"testing"
)

func TestValidateIsolation(t *testing.T) {
	for _, lvl := range []IsolationLevel{IsolationShared, IsolationIsolated, IsolationDedicated} {
		if err := ValidateIsolation(lvl); err != nil {
			t.Fatalf("%q must be valid: %v", lvl, err)
		}
	}
	for _, bad := range []IsolationLevel{"", "gvisor", "SHARED"} {
		if err := ValidateIsolation(bad); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}

func TestRequiresUpgradeNotice(t *testing.T) {
	if RequiresUpgradeNotice(IsolationShared) {
		t.Fatal("shared needs no upgrade notice")
	}
	if !RequiresUpgradeNotice(IsolationIsolated) || !RequiresUpgradeNotice(IsolationDedicated) {
		t.Fatal("isolated/dedicated must request an upgrade notice")
	}
}

func TestRenderedWorkloadStaysShared(t *testing.T) {
	spec := renderedWeb().Spec.Template.Spec
	if spec.RuntimeClassName != nil {
		t.Fatal("MVP must not attach any runtime class (no fake gVisor)")
	}
	if err := EnsureNoCrossPoolToleration(&spec); err != nil {
		t.Fatalf("shared rendering must stay tenant-pinned: %v", err)
	}
}
