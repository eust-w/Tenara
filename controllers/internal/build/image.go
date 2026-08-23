package build

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	registryAddr = "registry.tenara.local:5000"
	imageOrgPath = "tenara/apps"
	digestHeader = "Docker-Content-Digest"
)

// shaShort returns the 7-char short SHA used for immutable image tags (R3).
func shaShort(sha string) string {
	if len(sha) < 7 {
		return ""
	}
	return sha[:7]
}

// ImageTag returns the per-build immutable image reference
// registry.tenara.local:5000/tenara/apps/<app-id>:<sha-short>.
// An empty result means the build cannot be tagged (missing SHA).
func ImageTag(appID, sha string) string {
	short := shaShort(sha)
	if appID == "" || short == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s:%s", registryAddr, imageOrgPath, appID, short)
}

// BuildctlArgs returns the buildctl arguments building the workspace
// Dockerfile (frontend dockerfile.v0) and pushing to the local registry.
func BuildctlArgs(tag string) []string {
	return []string{
		"--addr", "unix:///run/buildkit/buildkitd.sock",
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=" + workspaceSrcPath,
		"--local", "dockerfile=" + workspaceSrcPath,
		"--output", "type=image,name=" + tag + ",push=true",
	}
}

// DigestFromRegistryHeader extracts the pushed manifest digest from a registry
// HEAD/GET response; empty means the push verification failed.
func DigestFromRegistryHeader(h http.Header) string {
	d := strings.TrimSpace(h.Get(digestHeader))
	if !strings.HasPrefix(d, "sha256:") {
		return ""
	}
	return d
}

// MarkFailed transitions the build to FAILED with a diagnostic reason and a
// reference pointing at the failing pod log.
func MarkFailed(b *Build, reason, message string) {
	b.Status.Phase = PhaseFailed
	b.Status.Reason = reason
	b.Status.Message = message
}
