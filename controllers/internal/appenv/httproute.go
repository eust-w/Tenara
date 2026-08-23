package appenv

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	GatewayName       = "tenara"
	frontendService   = "frontend"
	backendService    = "backend"
	apiPathPrefix     = "/api"
	rootPath          = "/"
	tlsCertAnnotation = "tenara.io/tls-cert"
)

// RoutingInput carries the AppSpec routing view needed to render one route.
type RoutingInput struct {
	Slug           string
	BaseDomain     string
	CertSecret     string
	FrontendPort   int32
	BackendPort    int32
	StripAPIPrefix bool
}

// HostnameFor derives the default tenant hostname <slug>.<baseDomain>.
func HostnameFor(slug, baseDomain string) string {
	return slug + "." + baseDomain
}

func parentRefMap() map[string]any {
	return map[string]any{"name": GatewayName, "namespace": GatewayNamespace}
}

func backendRefMap(svc string, port int32) map[string]any {
	return map[string]any{"name": svc, "port": port}
}

func pathMatch(value string) map[string]any {
	return map[string]any{
		"path": map[string]any{"type": "PathPrefix", "value": value},
	}
}

// RenderHTTPRoute renders the Gateway API HTTPRoute as an unstructured object
// (the typed gateway-api import is deferred until the module can be fetched):
// "/api" prefix → backend with optional prefix strip, "/" → frontend, default
// hostname <slug>.<baseDomain>. TLS terminates at the shared gateway whose
// certificate secret is referenced by annotation.
func RenderHTTPRoute(namespace string, in RoutingInput) *unstructured.Unstructured {
	r := &unstructured.Unstructured{}
	r.SetAPIVersion("gateway.networking.k8s.io/v1")
	r.SetKind("HTTPRoute")
	r.SetName("tenant")
	r.SetNamespace(namespace)
	r.SetLabels(map[string]string{LabelManagedBy: LabelManagedVal})
	r.SetAnnotations(map[string]string{tlsCertAnnotation: in.CertSecret})

	apiRule := map[string]any{
		"matches":     []any{pathMatch(apiPathPrefix)},
		"backendRefs": []any{backendRefMap(backendService, in.BackendPort)},
	}
	if in.StripAPIPrefix {
		apiRule["filters"] = []any{map[string]any{
			"type": "URLRewrite",
			"urlRewrite": map[string]any{
				"path": map[string]any{
					"type":               "ReplacePrefixMatch",
					"replacePrefixMatch": rootPath,
				},
			},
		}}
	}
	rootRule := map[string]any{
		"matches":     []any{pathMatch(rootPath)},
		"backendRefs": []any{backendRefMap(frontendService, in.FrontendPort)},
	}

	r.Object["spec"] = map[string]any{
		"parentRefs": []any{parentRefMap()},
		"hostnames":  []any{HostnameFor(in.Slug, in.BaseDomain)},
		"rules":      []any{apiRule, rootRule},
	}
	return r
}

// AppendVerifiedHostname adds a custom domain only after its DNS challenge
// was verified (todo20 linkage); unverified hosts are always rejected.
func AppendVerifiedHostname(route *unstructured.Unstructured, host string, verified bool) error {
	if !verified {
		return fmt.Errorf("hostname %q is not verified", host)
	}
	spec, ok := route.Object["spec"].(map[string]any)
	if !ok {
		return fmt.Errorf("route %q has no object spec", route.GetName())
	}
	hostnames, ok := spec["hostnames"].([]any)
	if !ok {
		return fmt.Errorf("route %q has no hostnames", route.GetName())
	}
	for _, h := range hostnames {
		if existing, isStr := h.(string); isStr && existing == host {
			return nil
		}
	}
	spec["hostnames"] = append(hostnames, host)
	return nil
}
