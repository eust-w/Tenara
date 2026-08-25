package provision

import (
	"encoding/json"
)

// AppEnvInput is everything the bridge needs to render one AppEnv manifest.
type AppEnvInput struct {
	AppID     string
	Env       string
	Name      string // slug used as the CR name
	QuotaTier string // free|pro
	Isolation string // shared|isolated|dedicated
}

// BuildAppEnv renders an unstructured tenara.io/v1 AppEnv manifest.
func BuildAppEnv(in AppEnvInput) Object {
	return map[string]any{
		"apiVersion": "tenara.io/v1",
		"kind":       "AppEnv",
		"metadata": map[string]any{
			"name": in.Name,
			"labels": map[string]any{
				"tenara.io/app-id": in.AppID,
				"tenara.io/env":    in.Env,
			},
		},
		"spec": map[string]any{
			"appId":     in.AppID,
			"env":       in.Env,
			"quotaTier": in.QuotaTier,
			"isolation": in.Isolation,
		},
	}
}

func marshal(obj Object) ([]byte, error) {
	return json.Marshal(obj)
}
