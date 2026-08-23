package build

import (
	"net/http"
	"strings"
	"testing"
)

func TestImageTagFormat(t *testing.T) {
	got := ImageTag("app-1", "abcdef1234567890")
	want := "registry.tenara.local:5000/tenara/apps/app-1:abcdef1"
	if got != want {
		t.Fatalf("ImageTag = %q, want %q", got, want)
	}
}

func TestImageTagNeverLatest(t *testing.T) {
	if got := ImageTag("app-1", ""); got != "" {
		t.Fatalf("missing SHA must yield empty tag, got %q", got)
	}
	if got := ImageTag("", "abcdef1234567890"); got != "" {
		t.Fatalf("missing app id must yield empty tag, got %q", got)
	}
	if got := ImageTag("app-1", "abcdef1234567890"); !strings.HasSuffix(got, ":abcdef1") {
		t.Fatalf("tag derives only from short SHA, got %q", got)
	}
}

func TestBuildctlArgs(t *testing.T) {
	tag := ImageTag("app-1", "abcdef1234567890")
	args := BuildctlArgs(tag)
	joined := strings.Join(args, "\n")

	for _, want := range []string{"dockerfile.v0", "context=" + workspaceSrcPath, "dockerfile=" + workspaceSrcPath, "push=true", "name=" + tag} {
		if !strings.Contains(joined, want) {
			t.Fatalf("buildctl args missing %q", want)
		}
	}
	if strings.Contains(joined, ":latest") {
		t.Fatal("buildctl args reference :latest")
	}
}

func TestDigestFromRegistryHeader(t *testing.T) {
	h := http.Header{}
	h.Set(digestHeader, "sha256:abc123")
	if got := DigestFromRegistryHeader(h); got != "sha256:abc123" {
		t.Fatalf("digest = %q", got)
	}
	h.Set(digestHeader, "garbage")
	if got := DigestFromRegistryHeader(h); got != "" {
		t.Fatalf("non-digest header must yield empty, got %q", got)
	}
	if got := DigestFromRegistryHeader(http.Header{}); got != "" {
		t.Fatalf("absent header must yield empty, got %q", got)
	}
}

func TestMarkFailed(t *testing.T) {
	b := sampleBuild()
	b.Status.Phase = PhaseBuilding
	MarkFailed(b, "digest-missing", "pod/b1 log ref")

	if b.Status.Phase != PhaseFailed || b.Status.Reason != "digest-missing" || b.Status.Message == "" {
		t.Fatalf("status = %+v", b.Status)
	}
}
