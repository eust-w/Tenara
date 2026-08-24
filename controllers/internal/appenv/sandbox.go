package appenv

import (
	corev1 "k8s.io/api/core/v1"
)

// Sandbox runtime classes injected for the isolated tier (RB§50 R14,
// D2-P2-1). Handlers must exist in tenant-pool containerd configs; pods
// requesting an absent class stay Pending and ResolveSandbox provides the
// documented degradation path.
const (
	SandboxClassGVisor = "gvisor"
	SandboxClassKata   = "kata-qemu"
)

// SandboxClassName maps an isolation level onto its runtime class. Shared
// renderings stay untouched; dedicated pins node pools instead (todo95).
func SandboxClassName(lvl IsolationLevel) string {
	if lvl == IsolationIsolated {
		return SandboxClassGVisor
	}
	return ""
}

// ApplySandboxClass attaches the runtime class for lvl when one applies;
// it is a no-op for levels rendered without a sandbox.
func ApplySandboxClass(spec *corev1.PodSpec, lvl IsolationLevel) {
	if class := SandboxClassName(lvl); class != "" {
		spec.RuntimeClassName = &class
	}
}

// ResolveSandbox picks the strongest available sandbox for lvl and reports
// whether the request had to degrade to shared semantics plus extra netpol
// isolation (design-doc fallback). Non-isolated levels never degrade.
func ResolveSandbox(available map[string]bool, lvl IsolationLevel) (string, bool) {
	if lvl != IsolationIsolated {
		return "", false
	}
	for _, class := range []string{SandboxClassGVisor, SandboxClassKata} {
		if available[class] {
			return class, false
		}
	}
	return "", true
}
