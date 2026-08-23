package build

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuilderPodSecurityBoundaries(t *testing.T) {
	pod := BuilderPod("test-builder", "tenara-build", nil)

	t.Run("automount SA token disabled", func(t *testing.T) {
		assertNoSAToken(t, pod)
	})
	t.Run("node selector targets build pool", func(t *testing.T) {
		assertBuildPoolSelector(t, pod)
	})
	t.Run("tolerates build taint", func(t *testing.T) {
		assertToleratesBuildTaint(t, pod)
	})
	t.Run("active deadline is 600s", func(t *testing.T) {
		assertDeadline600s(t, pod)
	})
	t.Run("container runs as UID 1000", func(t *testing.T) {
		assertRunAsUID1000(t, pod)
	})
	t.Run("no privileged", func(t *testing.T) {
		assertNotPrivileged(t, pod)
	})
	t.Run("no docker sock mount", func(t *testing.T) {
		assertNoDockerSock(t, pod)
	})
	t.Run("resource limits present", func(t *testing.T) {
		assertResourceLimits(t, pod)
	})
}

func assertNoSAToken(t *testing.T, p *corev1.Pod) {
	t.Helper()
	if p.Spec.AutomountServiceAccountToken == nil || *p.Spec.AutomountServiceAccountToken {
		t.Fatal("automountServiceAccountToken must be false")
	}
}

func assertBuildPoolSelector(t *testing.T, p *corev1.Pod) {
	t.Helper()
	v, ok := p.Spec.NodeSelector["tenara.io/role"]
	if !ok || v != "build" {
		t.Fatalf("nodeSelector = %v", p.Spec.NodeSelector)
	}
}

func assertToleratesBuildTaint(t *testing.T, p *corev1.Pod) {
	t.Helper()
	for _, tol := range p.Spec.Tolerations {
		if tol.Key == "tenara.io/role" && tol.Value == "build" {
			return
		}
	}
	t.Fatal("missing build pool toleration")
}

func assertDeadline600s(t *testing.T, p *corev1.Pod) {
	t.Helper()
	if p.Spec.ActiveDeadlineSeconds == nil || *p.Spec.ActiveDeadlineSeconds != 600 {
		t.Fatalf("deadline = %v, want 600", p.Spec.ActiveDeadlineSeconds)
	}
}

func assertRunAsUID1000(t *testing.T, p *corev1.Pod) {
	t.Helper()
	sc := p.Spec.SecurityContext
	if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != 1000 {
		t.Fatal("must run as UID 1000")
	}
}

func assertNotPrivileged(t *testing.T, p *corev1.Pod) {
	t.Helper()
	for _, c := range p.Spec.Containers {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			t.Fatalf("%s must not be privileged", c.Name)
		}
	}
}

func assertNoDockerSock(t *testing.T, p *corev1.Pod) {
	t.Helper()
	for _, c := range p.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if strings.Contains(vm.MountPath, "docker.sock") {
				t.Fatalf("docker.sock mounted in %s", c.Name)
			}
		}
	}
}

func assertResourceLimits(t *testing.T, p *corev1.Pod) {
	t.Helper()
	for _, c := range p.Spec.Containers {
		if len(c.Resources.Limits) == 0 {
			t.Fatalf("%s has no limits", c.Name)
		}
	}
}
