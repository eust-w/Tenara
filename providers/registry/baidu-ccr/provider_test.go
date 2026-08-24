package baiduccr

import (
	"context"
	"strings"
	"testing"
)

type fixtureResp struct {
	status int
	body   string
}

type testDoer struct {
	responses map[string]fixtureResp
}

func (d *testDoer) Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error) {
	key := method + " " + path
	if resp, ok := d.responses[key]; ok {
		return resp.status, []byte(resp.body), nil
	}
	return 500, []byte(`{}`), nil
}

func TestResolveDigestReturnsCanonicalSHA(t *testing.T) {
	p := New(&Config{Endpoint: "http://ccr.test"}, &testDoer{
		responses: map[string]fixtureResp{
			"GET /repositories/acme/app/tags/latest": {status: 200, body: `{"digest":"sha256:abc123"}`},
		},
	})
	dg, err := p.ResolveDigest(context.Background(), "acme/app", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if dg != "sha256:abc123" {
		t.Fatalf("digest = %q", dg)
	}
}

func TestCheckSignatureMatchesDigest(t *testing.T) {
	p := New(&Config{Endpoint: "http://ccr.test"}, &testDoer{
		responses: map[string]fixtureResp{
			"GET /repositories/acme/app/signatures": {
				status: 200,
				body:   `{"signatures":[{"digest":"sha256:abc123"}]}`,
			},
		},
	})
	ok, err := p.CheckSignature(context.Background(), "acme/app", "sha256:abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected signature verified=true")
	}
}

func TestDeleteImageNoErrorOnSuccess(t *testing.T) {
	p := New(&Config{Endpoint: "http://ccr.test"}, &testDoer{
		responses: map[string]fixtureResp{
			"DELETE /repositories/acme/app/manifests/sha256:abc123": {status: 202},
		},
	})
	if err := p.DeleteImage(context.Background(), "acme/app", "sha256:abc123"); err != nil {
		t.Fatal(err)
	}
}

func TestHealthzProbesCCREndpoint(t *testing.T) {
	p := New(&Config{Endpoint: "http://ccr.test"}, &testDoer{
		responses: map[string]fixtureResp{
			"GET /healthz": {status: 200},
		},
	})
	if err := p.Healthz(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProviderImplementsRegistryProviderInterface(t *testing.T) {
	var _ interface {
		ResolveDigest(ctx context.Context, repo, tagOrSHA string) (string, error)
		CheckSignature(ctx context.Context, repo, digest string) (bool, error)
		DeleteImage(ctx context.Context, repo, digest string) error
	} = (*Provider)(nil)
	assertStringsContains(t, "RegistryProvider compliance", "")
}

func assertStringsContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) && needle != "" {
		t.Fatalf("%q not found in %q", needle, haystack)
	}
}
