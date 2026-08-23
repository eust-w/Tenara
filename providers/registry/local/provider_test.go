package local

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tenara/providers/types"
)

var testDigest = "sha256:" + strings.Repeat("ab", 32)

type fakeRunner struct {
	respErr error
	gotArgs [][]string
}

func (f *fakeRunner) Run(_ context.Context, _ *Config, args []string) (string, error) {
	f.gotArgs = append(f.gotArgs, args)
	return "", f.respErr
}

func newTestProvider(ht *httptest.Server, f *fakeRunner) *Provider {
	return NewWith(Config{Endpoint: ht.URL, CosignKeyRef: "kms://stub/key"}, nil, f)
}

func recordingHandler(status int, digestHeader string, seen *[]*http.Request) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r)
		if digestHeader != "" {
			w.Header().Set("Docker-Content-Digest", digestHeader)
		}
		w.WriteHeader(status)
	}
}

func TestResolveDigestReturnsCanonicalDigest(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusOK, testDigest, &seen))
	defer srv.Close()

	p := newTestProvider(srv, &fakeRunner{})
	dg, err := p.ResolveDigest(context.Background(), "tenara/apps/app-1", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if dg != testDigest {
		t.Fatalf("digest = %q, want %q", dg, testDigest)
	}
	if len(seen) != 1 || seen[0].URL.Path != "/v2/tenara/apps/app-1/manifests/latest" {
		t.Fatalf("request path = %v", seen)
	}
	if !strings.Contains(seen[0].Header.Get("Accept"), acceptOCIIndex) {
		t.Fatalf("accept header missing oci index: %q", seen[0].Header.Get("Accept"))
	}
}

func TestResolveDigestRejectsMissingDigestHeader(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusOK, "", &seen))
	defer srv.Close()

	p := newTestProvider(srv, &fakeRunner{})
	if _, err := p.ResolveDigest(context.Background(), "repo", "latest"); err == nil ||
		!strings.Contains(err.Error(), "canonical digest") {
		t.Fatalf("missing header must be rejected as non-digest, got %v", err)
	}
}

func TestResolveDigestUnavailableOnServerError(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusInternalServerError, testDigest, &seen))
	defer srv.Close()

	p := newTestProvider(srv, &fakeRunner{})
	if _, err := p.ResolveDigest(context.Background(), "repo", "latest"); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestCheckSignatureGuardAndOutcomes(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusNotFound, "", &seen))
	defer srv.Close()

	f := &fakeRunner{}
	p := newTestProvider(srv, f)

	ok, cErr := p.CheckSignature(context.Background(), "tenara/apps/app-1", testDigest)
	if !ok || cErr != nil {
		t.Fatalf("signed image must verify, got (%v,%v)", ok, cErr)
	}
	args := f.gotArgs[0]
	wantRef := "tenara/apps/app-1@" + testDigest
	last := args[len(args)-1]
	if !strings.HasSuffix(last, wantRef) || !strings.Contains(last, "://") ||
		!strings.Contains(strings.Join(args, " "), "--allow-insecure-registry") {
		t.Fatalf("cosign args = %v", args)
	}

	unsigned := &fakeRunner{respErr: errors.New("exit status 1")}
	p2 := newTestProvider(srv, unsigned)
	ok2, cErr2 := p2.CheckSignature(context.Background(), "repo", testDigest)
	if ok2 || cErr2 != nil {
		t.Fatalf("unsigned must be (false,nil), got (%v,%v)", ok2, cErr2)
	}

	guarded := &fakeRunner{}
	p3 := newTestProvider(srv, guarded)
	if _, gErr := p3.CheckSignature(context.Background(), "repo", "latest"); gErr == nil {
		t.Fatal("tag reference must be rejected")
	}
	if len(guarded.gotArgs) != 0 {
		t.Fatal("digest guard must run before execution")
	}
}

func TestDeleteImageIssuesManifestDelete(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusAccepted, "", &seen))
	defer srv.Close()

	p := newTestProvider(srv, &fakeRunner{})
	if err := p.DeleteImage(context.Background(), "tenara/apps/app-1", testDigest); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].Method != http.MethodDelete ||
		seen[0].URL.Path != "/v2/tenara/apps/app-1/manifests/"+testDigest {
		t.Fatalf("delete request = %+v", seen)
	}

	badSrv := httptest.NewServer(recordingHandler(http.StatusForbidden, "", &seen))
	defer badSrv.Close()
	bad := newTestProvider(badSrv, &fakeRunner{})
	if err := bad.DeleteImage(context.Background(), "repo", testDigest); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestHealthzPingsV2Root(t *testing.T) {
	var seen []*http.Request
	srv := httptest.NewServer(recordingHandler(http.StatusOK, "", &seen))
	defer srv.Close()

	p := newTestProvider(srv, &fakeRunner{})
	if err := p.Healthz(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].URL.Path != "/v2/" {
		t.Fatalf("healthz request = %+v", seen)
	}

	downSrv := httptest.NewServer(recordingHandler(http.StatusServiceUnavailable, "", &seen))
	defer downSrv.Close()
	down := newTestProvider(downSrv, &fakeRunner{})
	if err := down.Healthz(context.Background()); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}
