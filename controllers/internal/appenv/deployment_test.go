package appenv

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func goodSvc() ServiceInput {
	return ServiceInput{
		Name:  "web",
		Image: "registry.tenara.local:5000/tenara/apps/app-1@sha256:abc123",
		Port:  8080,
	}
}

func TestRequireDigestImage(t *testing.T) {
	if err := RequireDigestImage(goodSvc().Image); err != nil {
		t.Fatalf("digest ref must pass: %v", err)
	}
	for _, bad := range []string{"", "nginx:latest", "registry.local/app:v1", "registry.local/app"} {
		if err := RequireDigestImage(bad); err == nil {
			t.Fatalf("non-digest ref %q must be rejected", bad)
		}
	}
}

func renderedWeb() *appsv1.Deployment {
	d, _ := RenderDeployment("acme", "prod", "app-acme-prod", goodSvc())
	return d
}

func TestRenderDeploymentContainerSecurity(t *testing.T) {
	sc := renderedWeb().Spec.Template.Spec.Containers[0].SecurityContext

	checks := []struct {
		name string
		ok   bool
	}{
		{"runAsNonRoot", sc.RunAsNonRoot != nil && *sc.RunAsNonRoot},
		{"allowPrivilegeEscalation false", sc.AllowPrivilegeEscalation != nil && !*sc.AllowPrivilegeEscalation},
		{"readOnlyRootFilesystem", sc.ReadOnlyRootFilesystem != nil && *sc.ReadOnlyRootFilesystem},
		{"capabilities drop ALL", sc.Capabilities != nil && len(sc.Capabilities.Drop) == 1 && sc.Capabilities.Drop[0] == "ALL"},
		{"seccomp RuntimeDefault", sc.SeccompProfile != nil && sc.SeccompProfile.Type == corev1.SeccompProfileTypeRuntimeDefault},
	}
	for _, tc := range checks {
		if !tc.ok {
			t.Errorf("%s violated", tc.name)
		}
	}
}

func TestRenderDeploymentPodDefaults(t *testing.T) {
	d := renderedWeb()
	spec := d.Spec.Template.Spec

	if spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken {
		t.Error("automount SA token must be false at pod level")
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 1 {
		t.Errorf("replicas default = %v, want 1", d.Spec.Replicas)
	}

	c := spec.Containers[0]
	for _, p := range []*corev1.Probe{c.ReadinessProbe, c.LivenessProbe} {
		if got := int32(p.TCPSocket.Port.IntValue()); got != 8080 {
			t.Fatalf("probe port = %d, want 8080", got)
		}
	}
}

func TestRenderDeploymentsFailFast(t *testing.T) {
	bad := goodSvc()
	bad.Image = "nginx:latest"

	_, err := RenderDeployments("acme", "prod", "ns", []ServiceInput{goodSvc(), bad})
	if err == nil {
		t.Fatal("any non-digest image must fail rendering")
	}
	if !strings.Contains(err.Error(), "latest") {
		t.Fatalf("error should mention latest: %v", err)
	}
}

func TestRenderDeploymentPinnedToTenantPool(t *testing.T) {
	d := renderedWeb()
	spec := d.Spec.Template.Spec

	if got := spec.NodeSelector[RoleNodeLabel]; got != TenantRoleValue {
		t.Fatalf("nodeSelector role = %q, want tenant", got)
	}

	var tenantTols int
	for _, tol := range spec.Tolerations {
		switch {
		case tol.Value == TenantRoleValue && tol.Effect == corev1.TaintEffectNoSchedule:
			tenantTols++
		case tol.Value == "build" || tol.Value == "management":
			t.Fatalf("cross-pool toleration leaked: %+v", tol)
		}
	}
	if tenantTols != 1 {
		t.Fatalf("tenant toleration count = %d, want exactly 1", tenantTols)
	}
	if err := EnsureNoCrossPoolToleration(&spec); err != nil {
		t.Fatalf("guard must pass on rendered workload: %v", err)
	}
}

func TestGuardRejectsForeignPoolToleration(t *testing.T) {
	spec := corev1.PodSpec{Tolerations: []corev1.Toleration{{
		Key: RoleNodeLabel, Operator: corev1.TolerationOpEqual,
		Value: "build", Effect: corev1.TaintEffectNoSchedule,
	}}}
	if err := EnsureNoCrossPoolToleration(&spec); err == nil {
		t.Fatal("build-pool toleration on tenant workload must be rejected")
	}
}
