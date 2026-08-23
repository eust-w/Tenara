package appenv

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestTenantServiceAccountTokenless(t *testing.T) {
	sa := TenantServiceAccount("app-acme-prod")
	if sa.Name != "tenant" || sa.Namespace != "app-acme-prod" {
		t.Fatalf("sa = %s/%s", sa.Namespace, sa.Name)
	}
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		t.Fatal("SA must not automount tokens (R14)")
	}
	if len(sa.Secrets) != 0 || len(sa.ImagePullSecrets) != 0 {
		t.Fatalf("SA must carry no secret wiring: %+v", sa)
	}
}

func TestFreeResourceQuotaFormula(t *testing.T) {
	cases := []struct {
		wantCPU  string
		wantMem  string
		services int
	}{
		{wantCPU: "250m", wantMem: "512Mi", services: 1},
		{wantCPU: "500m", wantMem: "1Gi", services: 2},
		{wantCPU: "1", wantMem: "2Gi", services: 4},
	}
	for _, tc := range cases {
		hard := FreeResourceQuota("ns", tc.services).Spec.Hard

		cpu := hard[corev1.ResourceLimitsCPU]
		if cpu.Cmp(resource.MustParse(tc.wantCPU)) != 0 {
			t.Errorf("svc=%d limits.cpu = %s, want %s", tc.services, cpu.String(), tc.wantCPU)
		}

		mem := hard[corev1.ResourceLimitsMemory]
		if mem.Cmp(resource.MustParse(tc.wantMem)) != 0 {
			t.Errorf("svc=%d limits.memory = %s, want %s", tc.services, mem.String(), tc.wantMem)
		}

		pods := hard[corev1.ResourcePods]
		if pods.String() != freeMaxPods {
			t.Errorf("svc=%d pods cap = %s, want %s", tc.services, pods.String(), freeMaxPods)
		}
	}

	clampedHard := FreeResourceQuota("ns", 0).Spec.Hard
	clamped := clampedHard[corev1.ResourceLimitsCPU]
	if clamped.Cmp(resource.MustParse("250m")) != 0 {
		t.Fatalf("zero services must clamp to one: %s", clamped.String())
	}
}

func TestDefaultLimitRangeValues(t *testing.T) {
	lr := DefaultLimitRange("ns")
	item := lr.Spec.Limits[0]
	if item.Type != corev1.LimitTypeContainer {
		t.Fatalf("type = %s", item.Type)
	}
	lists := []corev1.ResourceList{item.Default, item.DefaultRequest, item.Max}
	for _, rl := range lists {
		cpu := rl[corev1.ResourceCPU]
		if cpu.Cmp(resource.MustParse("250m")) != 0 {
			t.Errorf("cpu = %s, want 250m", cpu.String())
		}
		mem := rl[corev1.ResourceMemory]
		if mem.Cmp(resource.MustParse("256Mi")) != 0 {
			t.Errorf("memory = %s, want 256Mi", mem.String())
		}
	}
}
