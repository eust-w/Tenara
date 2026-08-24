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

func TestRenderedWorkloadStaysShared(t *testing.T) {
	spec := renderedWeb().Spec.Template.Spec
	if spec.RuntimeClassName != nil {
		t.Fatal("shared tier must not attach any runtime class")
	}
	if err := EnsureNoCrossPoolToleration(&spec); err != nil {
		t.Fatalf("shared rendering must stay tenant-pinned: %v", err)
	}
}
