package appenv

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DNSPort     = int32(53)
	BackendPort = int32(8000)
	MongoPort   = int32(27017)

	GatewayNamespace = "envoy-gateway-system"
	MongoNamespace   = "tenara-data"

	roleLabelKey       = "app"
	frontendRole       = "frontend"
	backendRole        = "backend"
	kubeSystemNSName   = "kube-system"
	kubeDNSAppLabel    = "k8s-app"
	kubeDNSAppValue    = "kube-dns"
	nsMetadataLabelKey = "kubernetes.io/metadata.name"
)

func protocolOf(p corev1.Protocol) *corev1.Protocol { return &p }

func portOf(num int32) *intstr.IntOrString {
	v := intstr.FromInt32(num)
	return &v
}

func basePolicy(name, namespace string, types []networkingv1.PolicyType) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: LabelManagedVal},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: types,
		},
	}
}

// DenyAllIngress selects every pod and admits no ingress at all.
func DenyAllIngress(namespace string) *networkingv1.NetworkPolicy {
	return basePolicy("deny-all-ingress", namespace, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress})
}

// DefaultEgress denies all egress except cluster DNS (udp/tcp 53). Future
// plans may append whitelisted peers; none ship in the MVP.
func DefaultEgress(namespace string, whitelist ...networkingv1.NetworkPolicyEgressRule) *networkingv1.NetworkPolicy {
	p := basePolicy("deny-all-egress", namespace, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress})
	p.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{
		To: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{nsMetadataLabelKey: kubeSystemNSName},
			},
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{kubeDNSAppLabel: kubeDNSAppValue},
			},
		}},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: protocolOf(corev1.ProtocolUDP), Port: portOf(DNSPort)},
			{Protocol: protocolOf(corev1.ProtocolTCP), Port: portOf(DNSPort)},
		},
	}}
	p.Spec.Egress = append(p.Spec.Egress, whitelist...)
	return p
}

// AllowGatewayIngress admits gateway-namespace traffic to frontend pods.
func AllowGatewayIngress(namespace string, frontendPort int32) *networkingv1.NetworkPolicy {
	p := basePolicy("allow-gateway", namespace, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress})
	p.Spec.PodSelector = metav1.LabelSelector{MatchLabels: map[string]string{roleLabelKey: frontendRole}}
	p.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
		From: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{nsMetadataLabelKey: GatewayNamespace},
			},
		}},
		Ports: []networkingv1.NetworkPolicyPort{{Port: portOf(frontendPort)}},
	}}
	return p
}

// AllowFrontendToBackend admits same-namespace frontend pods to backend:8000.
func AllowFrontendToBackend(namespace string) *networkingv1.NetworkPolicy {
	p := basePolicy("allow-internal", namespace, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress})
	p.Spec.PodSelector = metav1.LabelSelector{MatchLabels: map[string]string{roleLabelKey: backendRole}}
	p.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
		From: []networkingv1.NetworkPolicyPeer{{
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{roleLabelKey: frontendRole}},
		}},
		Ports: []networkingv1.NetworkPolicyPort{{Port: portOf(BackendPort)}},
	}}
	return p
}

// AllowBackendToMongo lets backend pods reach only this app's mongo pods
// (labeled tenara.io/app-id=<appID>) inside the data namespace.
func AllowBackendToMongo(namespace, appID string) *networkingv1.NetworkPolicy {
	p := basePolicy("allow-mongo", namespace, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress})
	p.Spec.PodSelector = metav1.LabelSelector{MatchLabels: map[string]string{roleLabelKey: backendRole}}
	p.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{
		To: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{nsMetadataLabelKey: MongoNamespace},
			},
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{LabelAppID: appID},
			},
		}},
		Ports: []networkingv1.NetworkPolicyPort{{Port: portOf(MongoPort)}},
	}}
	return p
}
