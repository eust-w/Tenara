package appenv

import (
	"testing"
)

func sampleRouting() RoutingInput {
	return RoutingInput{
		Slug: "acme", BaseDomain: "127.0.0.1.nip.io", CertSecret: "tenant-tls",
		FrontendPort: 3000, BackendPort: 8000, StripAPIPrefix: true,
	}
}

func renderedRoute() map[string]any {
	r := RenderHTTPRoute("app-acme-prod", sampleRouting())
	return r.Object["spec"].(map[string]any)
}

func TestHostnameFor(t *testing.T) {
	if got := HostnameFor("acme", "example.com"); got != "acme.example.com" {
		t.Fatalf("hostname = %q", got)
	}
}

func TestRouteMetadataAndHostnames(t *testing.T) {
	r := RenderHTTPRoute("app-acme-prod", sampleRouting())
	if r.GetKind() != "HTTPRoute" || r.GetName() != "tenant" || r.GetNamespace() != "app-acme-prod" {
		t.Fatalf("gvk/ns = %s/%s/%s", r.GetNamespace(), r.GetName(), r.GetKind())
	}
	if r.GetAnnotations()[tlsCertAnnotation] != "tenant-tls" {
		t.Fatalf("tls annotation = %v", r.GetAnnotations())
	}
	spec := renderedRoute()
	hns := spec["hostnames"].([]any)
	if len(hns) != 1 || hns[0].(string) != "acme.127.0.0.1.nip.io" {
		t.Fatalf("hostnames = %v", hns)
	}
	p0 := spec["parentRefs"].([]any)[0].(map[string]any)
	if p0["name"] != GatewayName || p0["namespace"] != GatewayNamespace {
		t.Fatalf("parentRef = %v", p0)
	}
}

func TestRouteRulesOrdering(t *testing.T) {
	rules := renderedRoute()["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want api-first + root", len(rules))
	}

	api := rules[0].(map[string]any)
	mv := api["matches"].([]any)[0].(map[string]any)["path"].(map[string]any)["value"]
	if mv != apiPathPrefix {
		t.Fatalf("first rule matches %v, want /api", mv)
	}
	br := api["backendRefs"].([]any)[0].(map[string]any)
	if br["name"] != backendService || br["port"] != int32(8000) {
		t.Fatalf("api backendRef = %v", br)
	}
	f0 := api["filters"].([]any)[0].(map[string]any)
	if f0["type"] != "URLRewrite" {
		t.Fatalf("filter type = %v", f0["type"])
	}
	pm := f0["urlRewrite"].(map[string]any)["path"].(map[string]any)
	if pm["replacePrefixMatch"] != rootPath {
		t.Fatalf("strip target = %v", pm["replacePrefixMatch"])
	}

	root := rules[1].(map[string]any)
	rv := root["matches"].([]any)[0].(map[string]any)["path"].(map[string]any)["value"]
	if rv != rootPath {
		t.Fatalf("root match = %v", rv)
	}
	rb := root["backendRefs"].([]any)[0].(map[string]any)
	if rb["name"] != frontendService || rb["port"] != int32(3000) {
		t.Fatalf("root backendRef = %v", rb)
	}
}

func TestNoStripOmitsFilter(t *testing.T) {
	in := sampleRouting()
	in.StripAPIPrefix = false
	spec := RenderHTTPRoute("ns", in).Object["spec"].(map[string]any)
	api := spec["rules"].([]any)[0].(map[string]any)
	if _, has := api["filters"]; has {
		t.Fatal("strip disabled must omit filters")
	}
}

func TestAppendVerifiedHostnameGuard(t *testing.T) {
	r := RenderHTTPRoute("ns", sampleRouting())
	before := len(r.Object["spec"].(map[string]any)["hostnames"].([]any))

	if err := AppendVerifiedHostname(r, "acme.dev.example.com", false); err == nil {
		t.Fatal("unverified hostname must be rejected")
	}
	if got := len(r.Object["spec"].(map[string]any)["hostnames"].([]any)); got != before {
		t.Fatal("rejected host must not be appended")
	}

	if err := AppendVerifiedHostname(r, "acme.dev.example.com", true); err != nil {
		t.Fatalf("verified append failed: %v", err)
	}
	if err := AppendVerifiedHostname(r, "acme.dev.example.com", true); err != nil {
		t.Fatalf("duplicate append must be idempotent: %v", err)
	}
	if got := len(r.Object["spec"].(map[string]any)["hostnames"].([]any)); got != before+1 {
		t.Fatalf("hostname count = %d, want %d", got, before+1)
	}
}
