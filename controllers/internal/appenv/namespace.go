package appenv

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelManagedBy  = "tenara.io/managed-by"
	LabelManagedVal = "tenara"
	LabelAppID      = "tenara.io/app-id"
	LabelEnv        = "tenara.io/env"
	PSSEnforceKey   = "pod-security.kubernetes.io/enforce"
	PSSAuditKey     = "pod-security.kubernetes.io/audit"
	PSSWarnKey      = "pod-security.kubernetes.io/warn"
	PSSRestricted   = "restricted"
)

// NamespaceName derives the per-app-env tenant namespace.
func NamespaceName(appID, env string) string {
	return fmt.Sprintf("app-%s-%s", appID, env)
}

// DesiredNamespace builds the tenant namespace spec with platform ownership
// labels and PSS enforce=restricted (R3).
func DesiredNamespace(appID, env string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: NamespaceName(appID, env),
			Labels: map[string]string{
				LabelManagedBy: LabelManagedVal,
				LabelAppID:     appID,
				LabelEnv:       env,
				PSSEnforceKey:  PSSRestricted,
				PSSAuditKey:    PSSRestricted,
				PSSWarnKey:     PSSRestricted,
			},
		},
	}
}

// EnsurePlatformOwned refuses to touch namespaces outside Tenara management.
func EnsurePlatformOwned(ns *corev1.Namespace) error {
	if ns.Labels[LabelManagedBy] != LabelManagedVal {
		return fmt.Errorf("namespace %s is not managed by %s", ns.Name, LabelManagedVal)
	}
	return nil
}
