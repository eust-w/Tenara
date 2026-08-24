package appenv

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	RoleNodeLabel   = "tenara.io/role"
	TenantRoleValue = "tenant"

	// DedicatedRoleValue names the exclusive node pool backing the
	// dedicated isolation tier (RB§39, todo95): its nodes carry the same
	// label/taint scheme so one mechanism pins either pool.
	DedicatedRoleValue = "dedicated"
)

// ApplyTenantScheduling pins a tenant pod spec to the tenant pool only:
// nodeSelector role=tenant plus its matching NoSchedule toleration. Builder
// and management pools are never referenced here.
func ApplyTenantScheduling(spec *corev1.PodSpec) {
	if spec.NodeSelector == nil {
		spec.NodeSelector = map[string]string{}
	}
	spec.NodeSelector[RoleNodeLabel] = TenantRoleValue
	spec.Tolerations = append(spec.Tolerations, corev1.Toleration{
		Key:      RoleNodeLabel,
		Operator: corev1.TolerationOpEqual,
		Value:    TenantRoleValue,
		Effect:   corev1.TaintEffectNoSchedule,
	})
}

// ApplyDedicatedScheduling pins a workload to the exclusive dedicated pool:
// nodeSelector role=dedicated plus its matching NoSchedule toleration. The
// pool's nodes are never shared with shared-tier tenants (RB§39).
func ApplyDedicatedScheduling(spec *corev1.PodSpec) {
	if spec.NodeSelector == nil {
		spec.NodeSelector = map[string]string{}
	}
	spec.NodeSelector[RoleNodeLabel] = DedicatedRoleValue
	spec.Tolerations = append(spec.Tolerations, corev1.Toleration{
		Key:      RoleNodeLabel,
		Operator: corev1.TolerationOpEqual,
		Value:    DedicatedRoleValue,
		Effect:   corev1.TaintEffectNoSchedule,
	})
}

// managedPools enumerates the pools tenant-side workloads may ever touch;
// builder and management pools remain forbidden (R3 blast-radius guard).
var managedPools = map[string]bool{
	TenantRoleValue: true, DedicatedRoleValue: true,
}

// EnsureNoCrossPoolToleration fails when a tenant-side workload carries a
// toleration for any pool outside the managed tenant/dedicated pair.
func EnsureNoCrossPoolToleration(spec *corev1.PodSpec) error {
	for _, tol := range spec.Tolerations {
		if tol.Key == RoleNodeLabel && !managedPools[tol.Value] {
			return fmt.Errorf("forbidden %q pool toleration on tenant workload", tol.Value)
		}
	}
	return nil
}
