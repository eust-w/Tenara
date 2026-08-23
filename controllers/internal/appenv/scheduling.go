package appenv

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	RoleNodeLabel   = "tenara.io/role"
	TenantRoleValue = "tenant"
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

// EnsureNoCrossPoolToleration fails when a tenant workload carries a
// toleration for any pool other than tenant (R3 blast-radius guard).
func EnsureNoCrossPoolToleration(spec *corev1.PodSpec) error {
	for _, tol := range spec.Tolerations {
		if tol.Key == RoleNodeLabel && tol.Value != TenantRoleValue {
			return fmt.Errorf("forbidden %q pool toleration on tenant workload", tol.Value)
		}
	}
	return nil
}
