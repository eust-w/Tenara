package appenv

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func podSpecFixture() *corev1.PodSpec {
	return &corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}
}

func TestSandboxClassName(t *testing.T) {
	if got := SandboxClassName(IsolationIsolated); got != SandboxClassGVisor {
		t.Fatalf("isolated -> %q, want %q", got, SandboxClassGVisor)
	}
	for _, lvl := range []IsolationLevel{IsolationShared, IsolationDedicated, ""} {
		if got := SandboxClassName(lvl); got != "" {
			t.Fatalf("%q must map to no class, got %q", lvl, got)
		}
	}
}

func TestApplySandboxClass(t *testing.T) {
	spec := podSpecFixture()
	ApplySandboxClass(spec, IsolationShared)
	if spec.RuntimeClassName != nil {
		t.Fatal("shared must not carry a runtime class")
	}
	ApplySandboxClass(spec, IsolationIsolated)
	if spec.RuntimeClassName == nil || *spec.RuntimeClassName != SandboxClassGVisor {
		t.Fatalf("isolated must pin %q", SandboxClassGVisor)
	}
}

func TestResolveSandbox(t *testing.T) {
	if cls, degraded := ResolveSandbox(map[string]bool{SandboxClassGVisor: true}, IsolationIsolated); degraded || cls != SandboxClassGVisor {
		t.Fatalf("gvisor available: got (%q,%v)", cls, degraded)
	}
	if cls, degraded := ResolveSandbox(map[string]bool{SandboxClassKata: true}, IsolationIsolated); degraded || cls != SandboxClassKata {
		t.Fatalf("kata-only: got (%q,%v)", cls, degraded)
	}
	if cls, degraded := ResolveSandbox(map[string]bool{}, IsolationIsolated); !degraded || cls != "" {
		t.Fatalf("no handler must degrade: got (%q,%v)", cls, degraded)
	}
	if _, degraded := ResolveSandbox(nil, IsolationShared); degraded {
		t.Fatal("shared never degrades")
	}
}
