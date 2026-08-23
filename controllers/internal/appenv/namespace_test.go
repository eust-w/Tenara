package appenv

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceName(t *testing.T) {
	if got := NamespaceName("acme", "prod"); got != "app-acme-prod" {
		t.Fatalf("name = %q", got)
	}
}

func TestDesiredNamespaceLabels(t *testing.T) {
	ns := DesiredNamespace("acme", "prod")
	want := map[string]string{
		LabelManagedBy: LabelManagedVal,
		LabelAppID:     "acme",
		LabelEnv:       "prod",
		PSSEnforceKey:  PSSRestricted,
		PSSAuditKey:    PSSRestricted,
		PSSWarnKey:     PSSRestricted,
	}
	for k, v := range want {
		if ns.Labels[k] != v {
			t.Fatalf("label %s = %q, want %q", k, ns.Labels[k], v)
		}
	}
	if ns.Name != "app-acme-prod" {
		t.Fatalf("ns name = %q", ns.Name)
	}
}

func TestEnsurePlatformOwned(t *testing.T) {
	foreign := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	if err := EnsurePlatformOwned(foreign); err == nil {
		t.Fatal("foreign namespace must be refused")
	}
	unlabeled := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
	if err := EnsurePlatformOwned(unlabeled); err == nil {
		t.Fatal("unlabeled namespace must be refused")
	}
	if err := EnsurePlatformOwned(DesiredNamespace("a", "p")); err != nil {
		t.Fatalf("platform namespace must be adoptable: %v", err)
	}
}
