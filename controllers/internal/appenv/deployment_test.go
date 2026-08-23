package appenv

import (
	"strings"
	"testing"

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

func TestRenderDeploymentHardening(t *testing.T) {
	d, err := RenderDeployment("acme", "prod", "app-acme-prod", goodSvc())
	if err != nil {
		t.Fatal(err)
	}
	c := d.Spec.Template.Spec.Containers[0]
	sc := c.SecurityContext

	switch {
	case sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot:
		t.Fatal("runAsNonRoot must be true")
	case sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation:
		t.Fatal("allowPrivilegeEscalation must be false")
	case sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem:
		t.Fatal("readOnlyRootFilesystem must be true")
	case sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL":
		t.Fatal("capabilities must drop ALL")
	case sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault:
		t.Fatal("seccomp must be RuntimeDefault")
	}

	spec := d.Spec.Template.Spec
	if spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken {
		t.Fatal("automount SA token must be false at pod level")
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 1 {
		t.Fatalf("replicas default = %v, want 1", d.Spec.Replicas)
	}

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
