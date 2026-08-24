package appenv

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestApplyDedicatedScheduling(t *testing.T) {
	spec := podSpecFixture()
	ApplyDedicatedScheduling(spec)
	if got := spec.NodeSelector[RoleNodeLabel]; got != DedicatedRoleValue {
		t.Fatalf("selector = %q, want %q", got, DedicatedRoleValue)
	}
	if err := EnsureNoCrossPoolToleration(spec); err != nil {
		t.Fatalf("dedicated pool must be managed: %v", err)
	}
}

func TestCrossPoolGuardStillBlocksForeignPools(t *testing.T) {
	spec := podSpecFixture()
	ApplyTenantScheduling(spec)
	spec.Tolerations = append(spec.Tolerations, corev1.Toleration{
		Key: RoleNodeLabel, Operator: corev1.TolerationOpEqual,
		Value: "builder", Effect: corev1.TaintEffectNoSchedule,
	})
	if err := EnsureNoCrossPoolToleration(spec); err == nil {
		t.Fatal("builder-pool toleration must stay forbidden")
	}
}
