package provision

import (
	"encoding/json"
)

// ServiceInput is one digest-pinned workload declared on the AppEnv.
type ServiceInput struct {
	Name  string
	Image string // must be digest-pinned
	Port  int32  // 0 = worker-style (no ports/probes)
}

// AppEnvInput is everything the bridge needs to render one AppEnv manifest.
type AppEnvInput struct {
	AppID     string
	Env       string
	Name      string // slug used as the CR name
	QuotaTier string // free|pro
	Isolation string // shared|isolated|dedicated
	Services  []ServiceInput
}

// BuildAppEnv renders an unstructured tenara.io/v1 AppEnv manifest.
func BuildAppEnv(in AppEnvInput) Object {
	spec := map[string]any{
		"appId":     in.AppID,
		"env":       in.Env,
		"quotaTier": in.QuotaTier,
		"isolation": in.Isolation,
	}
	if svcs := servicesToRaw(in.Services); svcs != nil {
		spec["services"] = svcs
	}
	return Object{
		"apiVersion": "tenara.io/v1",
		"kind":       "AppEnv",
		"metadata": map[string]any{
			"name": in.Name,
			"labels": map[string]any{
				"tenara.io/app-id": in.AppID,
				"tenara.io/env":    in.Env,
			},
		},
		"spec": spec,
	}
}

func servicesToRaw(svcs []ServiceInput) []any {
	if len(svcs) == 0 {
		return nil
	}
	out := make([]any, 0, len(svcs))
	for _, svc := range svcs {
		entry := map[string]any{"name": svc.Name, "image": svc.Image}
		if svc.Port > 0 {
			entry["port"] = svc.Port
		}
		out = append(out, entry)
	}
	return out
}

func marshal(obj Object) ([]byte, error) {
	return json.Marshal(obj)
}
