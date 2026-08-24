package domaincert

import (
	"encoding/json"
	"testing"
)

func TestRenderCertificateShape(t *testing.T) {
	raw, err := json.Marshal(RenderCertificate("d1", "app.example.com", "ns1"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if decErr := json.Unmarshal(raw, &m); decErr != nil {
		t.Fatal(decErr)
	}
	if m["apiVersion"] != APIVersionCM || m["kind"] != "Certificate" {
		t.Fatalf("bad gvk: %v/%v", m["apiVersion"], m["kind"])
	}
	meta := m["metadata"].(map[string]any)
	if meta["namespace"] != "ns1" || meta["name"] != "cert-d1" {
		t.Fatalf("bad metadata: %v", meta)
	}
	spec := m["spec"].(map[string]any)
	if spec["secretName"] != "tls-d1" {
		t.Fatalf("bad secretName: %v", spec["secretName"])
	}
	names := spec["dnsNames"].([]any)
	if len(names) != 1 || names[0] != "app.example.com" {
		t.Fatalf("bad dnsNames: %v", names)
	}
	ref := spec["issuerRef"].(map[string]any)
	if ref["name"] != IssuerName || ref["kind"] != "ClusterIssuer" {
		t.Fatalf("bad issuerRef: %v", ref)
	}
}

func TestRenderClusterIssuerShape(t *testing.T) {
	raw, err := json.Marshal(RenderClusterIssuer("https://acme.example/dir", "ops@tenara.io"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if decErr := json.Unmarshal(raw, &m); decErr != nil {
		t.Fatal(decErr)
	}
	acme := m["spec"].(map[string]any)["acme"].(map[string]any)
	if acme["server"] != "https://acme.example/dir" || acme["email"] != "ops@tenara.io" {
		t.Fatalf("bad acme fields: %v", acme)
	}
	solvers := acme["solvers"].([]any)
	if len(solvers) != 1 {
		t.Fatalf("want one solver: %v", solvers)
	}
	if _, ok := solvers[0].(map[string]any)["http01"]; !ok {
		t.Fatalf("solver must use http01")
	}
}
