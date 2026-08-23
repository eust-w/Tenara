package appenv

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestDenyAllIngressEmptyRules(t *testing.T) {
	p := DenyAllIngress("ns")
	if len(p.Spec.Ingress) != 0 {
		t.Fatalf("ingress rules = %d, want empty", len(p.Spec.Ingress))
	}
	if len(p.Spec.PolicyTypes) != 1 || p.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Fatalf("policyTypes = %v", p.Spec.PolicyTypes)
	}
}

func TestDefaultEgressOnlyDNS(t *testing.T) {
	p := DefaultEgress("ns")
	if len(p.Spec.Egress) != 1 {
		t.Fatalf("egress rules = %d, want exactly the DNS rule", len(p.Spec.Egress))
	}
	r := p.Spec.Egress[0]
	if len(r.To) != 1 || r.To[0].PodSelector.MatchLabels[kubeDNSAppLabel] != kubeDNSAppValue {
		t.Fatalf("dns peer = %+v", r.To)
	}
	if r.To[0].NamespaceSelector.MatchLabels[nsMetadataLabelKey] != kubeSystemNSName {
		t.Fatal("dns peer must target kube-system namespace")
	}

	var udpOK, tcpOK bool
	for _, pp := range r.Ports {
		if pp.Port.IntValue() != int(DNSPort) {
			t.Fatalf("port = %v, want 53", pp.Port)
		}
		if *pp.Protocol == corev1.ProtocolUDP {
			udpOK = true
		}
		if *pp.Protocol == corev1.ProtocolTCP {
			tcpOK = true
		}
	}
	if !udpOK || !tcpOK {
		t.Fatalf("want udp+tcp 53, got udp=%v tcp=%v", udpOK, tcpOK)
	}

	widened := DefaultEgress("ns", networkingv1.NetworkPolicyEgressRule{})
	if len(widened.Spec.Egress) != 2 {
		t.Fatalf("whitelist interface must append rules, got %d", len(widened.Spec.Egress))
	}
}

func TestAllowChainSelectorsAndPorts(t *testing.T) {
	gw := AllowGatewayIngress("tenant", 3000)
	if gw.Spec.PodSelector.MatchLabels[roleLabelKey] != frontendRole {
		t.Fatalf("gw selects %v", gw.Spec.PodSelector)
	}
	if got := gw.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels[nsMetadataLabelKey]; got != GatewayNamespace {
		t.Fatalf("gateway ns = %q", got)
	}
	if got := gw.Spec.Ingress[0].Ports[0].Port.IntValue(); got != 3000 {
		t.Fatalf("frontend port = %d, want 3000", got)
	}

	fb := AllowFrontendToBackend("tenant")
	if fb.Spec.PodSelector.MatchLabels[roleLabelKey] != backendRole {
		t.Fatal("must select backend pods")
	}
	from := fb.Spec.Ingress[0].From[0]
	if from.NamespaceSelector != nil || from.PodSelector.MatchLabels[roleLabelKey] != frontendRole {
		t.Fatalf("same-ns frontend peer expected: %+v", from)
	}
	if got := fb.Spec.Ingress[0].Ports[0].Port.IntValue(); got != int(BackendPort) {
		t.Fatalf("backend port = %d, want %d", got, BackendPort)
	}

	bm := AllowBackendToMongo("tenant", "acme")
	to := bm.Spec.Egress[0].To[0]
	if to.NamespaceSelector.MatchLabels[nsMetadataLabelKey] != MongoNamespace ||
		to.PodSelector.MatchLabels[LabelAppID] != "acme" {
		t.Fatalf("mongo peer = %+v", to)
	}
	if got := bm.Spec.Egress[0].Ports[0].Port.IntValue(); got != int(MongoPort) {
		t.Fatalf("mongo port = %d, want %d", got, MongoPort)
	}
}

func TestNoWildcardOrMetadataTargets(t *testing.T) {
	policies := []*networkingv1.NetworkPolicy{
		DenyAllIngress("ns"),
		DefaultEgress("ns"),
		AllowGatewayIngress("ns", 80),
		AllowFrontendToBackend("ns"),
		AllowBackendToMongo("ns", "a"),
	}
	var blob strings.Builder
	enc := json.NewEncoder(&blob)
	for _, p := range policies {
		if encErr := enc.Encode(p); encErr != nil {
			t.Fatal(encErr)
		}
	}
	for _, banned := range []string{"169.254", "0.0.0.0/0", "::/0"} {
		if strings.Contains(blob.String(), banned) {
			t.Fatalf("policy set contains banned target %q", banned)
		}
	}
}
