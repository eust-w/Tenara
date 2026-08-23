package appenv

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func hardenedSC() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(true),
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

func TestDetectReplicaDriftRestoresDesired(t *testing.T) {
	if drifted, fix := DetectReplicaDrift(2, 1); !drifted || fix != 2 {
		t.Fatalf("scaled-down workload must flag drift, got (%v,%d)", drifted, fix)
	}
	if drifted, _ := DetectReplicaDrift(2, 2); drifted {
		t.Fatal("matching replicas must not flag drift")
	}
}

func TestSecurityDriftPinpointsEachField(t *testing.T) {
	desired := hardenedSC()

	live := hardenedSC()
	live.RunAsNonRoot = boolPtr(false)
	live.ReadOnlyRootFilesystem = boolPtr(false)
	got := SecurityDrift(desired, live)
	if len(got) != 2 ||
		!strings.Contains(strings.Join(got, ","), "runAsNonRoot") ||
		!strings.Contains(strings.Join(got, ","), "readOnlyRootFilesystem") {
		t.Fatalf("drift = %v", got)
	}

	compliant := SecurityDrift(desired, hardenedSC())
	if len(compliant) != 0 {
		t.Fatalf("compliant workload reported drift: %v", compliant)
	}

	nilDrift := SecurityDrift(desired, nil)
	if len(nilDrift) != 1 || nilDrift[0] != "securityContext" {
		t.Fatalf("nil live context must count as full drift, got %v", nilDrift)
	}
}

func TestMissingNetPolsRecreationList(t *testing.T) {
	expected := []string{"deny-all-ingress", "deny-all-egress", "allow-gateway"}
	live := map[string]bool{
		"deny-all-ingress": true,
		"deny-all-egress":  true,
	}
	got := MissingNetPols(expected, live)
	if len(got) != 1 || got[0] != "allow-gateway" {
		t.Fatalf("missing = %v, want allow-gateway recreated", got)
	}
	if len(MissingNetPols(expected, map[string]bool{
		"deny-all-ingress": true, "deny-all-egress": true, "allow-gateway": true,
	})) != 0 {
		t.Fatal("fully present set must yield no recreation")
	}
}

func TestOwnedByUsBlocksThirdPartyMutation(t *testing.T) {
	if !OwnedByUs(map[string]string{LabelManagedBy: LabelManagedVal}) {
		t.Fatal("tenara-managed object must be owned")
	}
	if OwnedByUs(map[string]string{}) {
		t.Fatal("unlabelled object must never be touched")
	}
	if OwnedByUs(map[string]string{LabelManagedBy: "someone-else"}) {
		t.Fatal("foreign-managed object must never be touched")
	}
}

func TestReportDriftStampsControllerAudit(t *testing.T) {
	at := time.Unix(1700000000, 0)
	rep := ReportDrift("acme", "app-acme-prod", []DriftAction{
		{Name: "web", Kind: "deployment", Field: "replicas", Desired: "2"},
	}, at)
	if rep.ActorType != "controller" || rep.AuditedAt != at || len(rep.Actions) != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.AppID != "acme" || rep.Namespace != "app-acme-prod" {
		t.Fatalf("identity = %s/%s", rep.Namespace, rep.AppID)
	}
}
