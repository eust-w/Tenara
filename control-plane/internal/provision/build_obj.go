package provision

// BuildInput carries what the bridge knows when materializing a Build CR
// before any image exists (phase 2 entry point).
type BuildInput struct {
	AppID      string
	Env        string
	Name       string // generated CR name (e.g. <slug>-<shortid>)
	GitURL     string
	GitSHA     string // optional pin; empty = default branch HEAD
	Dockerfile string // optional override
}

// BuildBuild renders an unstructured tenara.io/v1 Build manifest.
func BuildBuild(in BuildInput) Object {
	git := map[string]any{"url": in.GitURL}
	if in.GitSHA != "" {
		git["sha"] = in.GitSHA
	}
	spec := map[string]any{
		"app": in.AppID,
		"env": in.Env,
		"git": git,
	}
	if in.Dockerfile != "" {
		spec["dockerfile"] = in.Dockerfile
	}
	obj := Object{
		"apiVersion": "tenara.io/v1",
		"kind":       "Build",
		"metadata": map[string]any{
			"name": in.Name,
			"labels": map[string]any{
				"tenara.io/app-id": in.AppID,
				"tenara.io/env":    in.Env,
			},
		},
		"spec": spec,
	}
	return obj
}
