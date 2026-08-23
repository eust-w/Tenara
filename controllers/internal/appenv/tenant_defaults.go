package appenv

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Free-tier budgets from RB§29; the quota source of truth is the approved
// plan (todo18), which passes its service count here.
const (
	freeMaxPods       = "3"
	freeCPUPerService = "250m"
	freeMemPerService = "512Mi"
	defaultCPU        = "250m"
	defaultMem        = "256Mi"
)

// TenantServiceAccount returns the namespace-scoped SA for one app env. It
// never receives RoleBindings or ClusterRoleBindings anywhere in the
// codebase: workloads run token-less and permission-less (R14).
func TenantServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: LabelManagedVal},
		},
		AutomountServiceAccountToken: boolPtr(false),
	}
}

// FreeResourceQuota scales the free-tier CPU/memory ceilings linearly with
// the planned service count while capping pods at 3.
func FreeResourceQuota(namespace string, serviceCount int) *corev1.ResourceQuota {
	if serviceCount < 1 {
		serviceCount = 1
	}

	cpu := resource.MustParse(freeCPUPerService)
	mem := resource.MustParse(freeMemPerService)
	for range serviceCount - 1 {
		cpu.Add(resource.MustParse(freeCPUPerService))
		mem.Add(resource.MustParse(freeMemPerService))
	}

	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-free",
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: LabelManagedVal},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{ //nolint:exhaustive // free-tier subset by design
				corev1.ResourcePods:         resource.MustParse(freeMaxPods),
				corev1.ResourceLimitsCPU:    cpu,
				corev1.ResourceLimitsMemory: mem,
			},
		},
	}
}

// DefaultLimitRange injects per-container defaults and matching maxima so a
// single container cannot exceed its own free-tier share.
func DefaultLimitRange(namespace string) *corev1.LimitRange {
	cpu := resource.MustParse(defaultCPU)
	mem := resource.MustParse(defaultMem)
	list := corev1.ResourceList{ //nolint:exhaustive // cpu/mem pair by design
		corev1.ResourceCPU:    cpu,
		corev1.ResourceMemory: mem,
	}

	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-defaults",
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: LabelManagedVal},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{{
				Type:           corev1.LimitTypeContainer,
				Default:        list, //nolint:gocritic // shared value intentional
				DefaultRequest: list, //nolint:gocritic // shared value intentional
				Max:            list, //nolint:gocritic // shared value intentional
			}},
		},
	}
}
