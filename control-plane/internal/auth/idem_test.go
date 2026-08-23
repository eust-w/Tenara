package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/pgstore"
)

func authedRaw(t *testing.T, method, url, bearer, idemKey string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		reader = bytes.NewReader(raw)
	}
	req, reqErr := http.NewRequest(method, url, reader)
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	raw, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return res, raw
}

func newIdemHarness(t *testing.T) (*httptest.Server, *pgxpool.Pool, string) {
	ts := newTestServer(t)
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)
	jwt := registerVerifiedUser(t, ts, "t14")
	return ts, pool, jwt
}

func TestIdemReplayIdentical(t *testing.T) {
	ts, pool, jwt := newIdemHarness(t)
	key, keyErr := RandomToken()
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	name := fmt.Sprintf("idem-a-%d", timeNowUnix())
	res1, body1 := authedRaw(t, http.MethodPost, ts.URL+"/v1/tokens", jwt, key,
		map[string]string{"name": name})
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first = %d body=%s", res1.StatusCode, body1)
	}
	if res1.Header.Get("Idempotent-Replayed") == "true" {
		t.Fatal("first call must not be marked replayed")
	}

	res2, body2 := authedRaw(t, http.MethodPost, ts.URL+"/v1/tokens", jwt, key,
		map[string]string{"name": name})
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("replay = %d", res2.StatusCode)
	}
	if res2.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("replay marker missing")
	}
	if !bytes.Equal(body1, body2) {
		t.Fatalf("replayed body differs:\n%s\n---\n%s", body1, body2)
	}

	var count int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM api_tokens WHERE name = $1`, name).Scan(&count); scanErr != nil {
		t.Fatal(scanErr)
	}
	if count != 1 {
		t.Fatalf("created %d rows, want 1", count)
	}
}

func TestIdemConflict(t *testing.T) {
	ts, _, jwt := newIdemHarness(t)
	key, _ := RandomToken()
	stamp := timeNowUnix()
	res1, _ := authedRaw(t, http.MethodPost, ts.URL+"/v1/tokens", jwt, key,
		map[string]string{"name": fmt.Sprintf("conflict-a-%d", stamp)})
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("first = %d", res1.StatusCode)
	}
	res2, body2 := authedRaw(t, http.MethodPost, ts.URL+"/v1/tokens", jwt, key,
		map[string]string{"name": fmt.Sprintf("conflict-b-%d", stamp)})
	if res2.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("mismatch = %d body=%s, want 422", res2.StatusCode, body2)
	}
	if !strings.Contains(string(body2), "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("missing error code: %s", body2)
	}
}

func TestIdemGetBypass(t *testing.T) {
	ts, _, jwt := newIdemHarness(t)
	key, _ := RandomToken()
	res1, _ := authedRaw(t, http.MethodGet, ts.URL+"/v1/me", jwt, key, nil)
	if res1.StatusCode != http.StatusOK {
		t.Fatalf("get1 = %d", res1.StatusCode)
	}
	res2, _ := authedRaw(t, http.MethodGet, ts.URL+"/v1/me", jwt, key, nil)
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("get2 = %d", res2.StatusCode)
	}
	if res2.Header.Get("Idempotent-Replayed") == "true" {
		t.Fatal("GET must bypass idempotency")
	}
}

func TestIdemExpiryPurge(t *testing.T) {
	_, pool, _ := newIdemHarness(t)
	var orgID string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT id FROM organizations LIMIT 1`).Scan(&orgID); scanErr != nil {
		t.Skipf("no organizations yet: %v", scanErr)
	}
	expiredKey := fmt.Sprintf("expired-%d", timeNowUnix())
	if _, execErr := pool.Exec(context.Background(),
		`INSERT INTO idempotency_keys (idempotency_key, org_id, request_hash, expires_at)
		 VALUES ($1, $2, 'stale', now() - interval '1 hour')`,
		expiredKey, orgID); execErr != nil {
		t.Fatal(execErr)
	}
	store := &Store{pool: pool}
	purged, purgeErr := store.CleanupExpiredIdempotency(context.Background())
	if purgeErr != nil {
		t.Fatal(purgeErr)
	}
	if purged < 1 {
		t.Fatal("expected at least one purged row")
	}
	var count int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM idempotency_keys WHERE idempotency_key = $1`, expiredKey).Scan(&count); scanErr != nil {
		t.Fatal(scanErr)
	}
	if count != 0 {
		t.Fatal("expired row survived cleanup")
	}
}
