package appenv

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestTenantNamespacePSSEnforced locks the MVP policy-engine decision (R3):
// built-in Pod Security Standard enforce=restricted on every tenant
// namespace; no admission engine ships in the MVP.
func TestTenantNamespacePSSEnforced(t *testing.T) {
	ns := DesiredNamespace("acme", "prod")
	for _, key := range []string{PSSEnforceKey, PSSAuditKey, PSSWarnKey} {
		if got := ns.Labels[key]; got != PSSRestricted {
			t.Fatalf("label %s = %q, want %q", key, got, PSSRestricted)
		}
	}
}

// TestNoPrivilegedWorkloadEverRendered proves the renderer cannot emit a
// workload that PSS restricted would reject; the apiserver denial path only
// ever triggers on foreign manifests submitted into the tenant namespace.
func TestNoPrivilegedWorkloadEverRendered(t *testing.T) {
	svcs := []ServiceInput{
		{Name: "web", Image: "registry.tenara.local:5000/tenara/apps/a@sha256:aa", Port: 3000},
		{Name: "api", Image: "registry.tenara.local:5000/tenara/apps/b@sha256:bb", Port: 8000, Replicas: 2},
	}
	deps, err := RenderDeployments("acme", "prod", "app-acme-prod", svcs)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range deps {
		spec := d.Spec.Template.Spec
		if spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken {
			t.Fatalf("%s: SA token mount must stay off", d.Name)
		}
		for _, c := range spec.Containers {
			assertRestrictedContainer(t, d.Name, c)
		}
	}
}

func assertRestrictedContainer(t *testing.T, dep string, c corev1.Container) {
	t.Helper()
	sc := c.SecurityContext
	switch {
	case sc == nil:
		t.Fatalf("%s/%s missing securityContext", dep, c.Name)
	case sc.Privileged != nil && *sc.Privileged:
		t.Fatalf("%s/%s privileged container forbidden", dep, c.Name)
	case sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation:
		t.Fatalf("%s/%s must forbid escalation", dep, c.Name)
	case sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem:
		t.Fatalf("%s/%s must use read-only root filesystem", dep, c.Name)
	case sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot:
		t.Fatalf("%s/%s must run as non-root", dep, c.Name)
	case sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault:
		t.Fatalf("%s/%s seccomp must be RuntimeDefault", dep, c.Name)
	}
}
