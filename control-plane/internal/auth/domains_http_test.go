package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/apps"
	"tenara/control-plane/internal/kms"
	"tenara/control-plane/internal/pgstore"
)

const txtPrefix = "_tenara-challenge."

type fakeTXTResolver struct {
	txt map[string][]string
	mu  sync.Mutex
}

func (f *fakeTXTResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.txt[strings.TrimSuffix(name, ".")]; ok {
		return append([]string(nil), v...), nil
	}
	return nil, fmt.Errorf("no TXT record for %s", name)
}

func seedTXT(f *fakeTXTResolver, host, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txt[txtPrefix+host] = []string{token}
}

// Local harness exposing the mounted apps handlers for seam injection.
func newDomainsTestServer(t *testing.T) (*httptest.Server, *apps.Handlers, *pgxpool.Pool) {
	t.Helper()
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	svc := NewService(NewStore(pool),
		NewTokenManager("test-secret-key-32-bytes-long!!"), "http://localhost:8080")
	kmsStub, kmsErr := kms.NewStub(strings.Repeat("ab", 32))
	if kmsErr != nil {
		t.Fatal(kmsErr)
	}
	appsH := apps.New(pool, NewBridge(svc), "127.0.0.1.nip.io", kmsStub)

	r := chi.NewRouter()
	svc.Mount(r)
	appsH.Mount(r)
	return httptest.NewServer(r), appsH, pool
}

func TestDomainsHTTP(t *testing.T) {
	ts, appsH, pool := newDomainsTestServer(t)
	fake := &fakeTXTResolver{txt: map[string][]string{}}
	appsH.SetDNSResolver(fake)

	jwt := registerVerifiedUser(t, ts, "t20")
	stamp := timeNowUnix()
	appID := createNamedApp(t, ts, jwt, fmt.Sprintf("domapp-%d", stamp))

	t.Run("default subdomain instantly verified", func(t *testing.T) {
		runDefaultSubdomain(t, ts, jwt, appID)
	})
	t.Run("custom domain pending then verified via TXT", func(t *testing.T) {
		runCustomTXTFlow(t, ts, jwt, appID, fake, stamp)
	})
	t.Run("duplicate hostname conflicts", func(t *testing.T) {
		runDuplicateHostname(t, ts, jwt, appID, stamp)
	})
	t.Run("unverified domain cannot bind routing", func(t *testing.T) {
		runUnverifiedBindRule(t, ts, pool, jwt, appID, stamp)
	})
}

func runDefaultSubdomain(t *testing.T, ts *httptest.Server, jwt, appID string) {
	t.Helper()
	code, body := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/domains", jwt, map[string]string{})
	if code != http.StatusCreated {
		t.Fatalf("allocate = %d body=%s", code, body)
	}
	var d struct {
		Hostname  string `json:"hostname"`
		Verified  bool   `json:"verified"`
		IsDefault bool   `json:"is_default"`
	}
	if decodeErr := json.Unmarshal(body, &d); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !d.Verified || !d.IsDefault ||
		!strings.HasSuffix(d.Hostname, ".127.0.0.1.nip.io") {
		t.Fatalf("unexpected default domain: %+v", d)
	}
}

func runCustomTXTFlow(
	t *testing.T, ts *httptest.Server, jwt, appID string,
	fake *fakeTXTResolver, stamp int64,
) {
	t.Helper()
	host := fmt.Sprintf("app-%d.example.com", stamp)
	code, body := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/domains", jwt,
		map[string]string{"hostname": host})
	if code != http.StatusCreated {
		t.Fatalf("add custom = %d body=%s", code, body)
	}
	var created struct {
		ID           string `json:"id"`
		TXTChallenge string `json:"txt_challenge"`
		Verified     bool   `json:"verified"`
	}
	if decodeErr := json.Unmarshal(body, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if created.Verified || created.TXTChallenge == "" {
		t.Fatalf("expected pending with challenge: %+v", created)
	}

	verifyURL := ts.URL + "/v1/apps/" + appID + "/domains/" + created.ID + "/verify"
	pendingCode, pendingBody := authedJSON(t, http.MethodPost, verifyURL, jwt, nil)
	if pendingCode != http.StatusOK {
		t.Fatalf("verify#1 = %d body=%s", pendingCode, pendingBody)
	}
	var stillPending struct {
		Verified bool `json:"verified"`
	}
	if decodeErr := json.Unmarshal(pendingBody, &stillPending); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if stillPending.Verified {
		t.Fatal("must stay pending without TXT record")
	}

	seedTXT(fake, host, created.TXTChallenge)
	okCode, okBody := authedJSON(t, http.MethodPost, verifyURL, jwt, nil)
	if okCode != http.StatusOK {
		t.Fatalf("verify#2 = %d body=%s", okCode, okBody)
	}
	var done struct {
		Verified bool `json:"verified"`
	}
	if decodeErr := json.Unmarshal(okBody, &done); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !done.Verified {
		t.Fatalf("TXT hit must verify: %s", okBody)
	}
}

func runDuplicateHostname(
	t *testing.T, ts *httptest.Server, jwt, appID string, stamp int64,
) {
	t.Helper()
	host := fmt.Sprintf("dup-%d.example.com", stamp)
	code1, body1 := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/domains", jwt,
		map[string]string{"hostname": host})
	if code1 != http.StatusCreated {
		t.Fatalf("first add = %d body=%s", code1, body1)
	}
	code2, _ := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/domains", jwt,
		map[string]string{"hostname": host})
	if code2 != http.StatusConflict {
		t.Fatalf("duplicate = %d, want 409", code2)
	}
}

func runUnverifiedBindRule(
	t *testing.T, ts *httptest.Server, pool *pgxpool.Pool, jwt, appID string,
	stamp int64,
) {
	t.Helper()
	pendCode, pendBody := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/domains", jwt,
		map[string]string{"hostname": fmt.Sprintf("pending-%d.example.com", stamp)})
	if pendCode != http.StatusCreated {
		t.Fatalf("add pending = %d body=%s", pendCode, pendBody)
	}
	var created struct {
		ID string `json:"id"`
	}
	if decodeErr := json.Unmarshal(pendBody, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}

	tm := NewTokenManager("test-secret-key-32-bytes-long!!")
	userID, parseErr := tm.Parse(jwt)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	aStore := &Store{pool: pool}
	orgA, orgErr := aStore.DefaultOrgForUser(context.Background(), userID)
	if orgErr != nil {
		t.Fatal(orgErr)
	}
	appStore := apps.NewStore(pool)
	_, conflictErr := appStore.RequireVerifiedDomain(
		context.Background(), orgA, appID, created.ID)
	if !errors.Is(conflictErr, apps.ErrConflict) {
		t.Fatalf("unverified bind = %v, want ErrConflict (409 semantics at route layer)", conflictErr)
	}
}
