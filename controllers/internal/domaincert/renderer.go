// Package domaincert renders cert-manager resources backing automatic TLS
// for custom domains (D2-P2-2 / todo89). Rendering is pure and unit-tested;
// live issuance additionally requires cert-manager installed plus an ACME
// account (design-doc dependency gate).
package domaincert

const (
	// APIVersionCM pins the consumed cert-manager API group/version.
	APIVersionCM = "cert-manager.io/v1"
	// IssuerName is the platform-wide ClusterIssuer reference.
	IssuerName = "tenara-acme"
)

// RenderCertificate builds a Certificate manifest for one verified custom
// domain. The issued secret derives from the immutable domain id so renewals
// stay stable across AppEnv churn.
func RenderCertificate(domainID, hostname, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": APIVersionCM,
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "cert-" + domainID,
			"namespace": namespace,
			"labels":    map[string]any{"tenara.io/domain-id": domainID},
		},
		"spec": map[string]any{
			"secretName": "tls-" + domainID,
			"dnsNames":   []string{hostname},
			"issuerRef":  map[string]any{"name": IssuerName, "kind": "ClusterIssuer"},
		},
	}
}

// RenderClusterIssuer builds the ACME ClusterIssuer wired to HTTP-01 via the
// platform ingress. Server and contact email come from deployment config so
// staging versus production is purely an ops choice.
func RenderClusterIssuer(acmeServer, acmeEmail string) map[string]any {
	return map[string]any{
		"apiVersion": APIVersionCM,
		"kind":       "ClusterIssuer",
		"metadata":   map[string]any{"name": IssuerName},
		"spec": map[string]any{
			"acme": map[string]any{
				"server":              acmeServer,
				"email":               acmeEmail,
				"privateKeySecretRef": map[string]any{"name": IssuerName + "-account-key"},
				"solvers": []any{map[string]any{
					"http01": map[string]any{"ingress": map[string]any{}},
				}},
			},
		},
	}
}
