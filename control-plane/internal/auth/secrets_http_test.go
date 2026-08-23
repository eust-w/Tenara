package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/pgstore"
)

func putEnv(t *testing.T, ts *httptest.Server, jwt, appID, name, value string) {
	t.Helper()
	code, body := authedJSON(t, http.MethodPut, ts.URL+"/v1/apps/"+appID+"/env", jwt,
		map[string]string{"name": name, "value": value})
	if code != http.StatusNoContent {
		t.Fatalf("put env %s = %d body=%s", name, code, body)
	}
}

func TestSecretsHTTP(t *testing.T) {
	ts := newTestServer(t)
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	jwtOwner := registerVerifiedUser(t, ts, "t19owner")
	stamp := timeNowUnix()
	appID := createNamedApp(t, ts, jwtOwner, fmt.Sprintf("secapp-%d", stamp))
	realDB := fmt.Sprintf("postgres://real-db-%d", stamp)
	realKey := fmt.Sprintf("sk-real-key-%d", stamp)

	putEnv(t, ts, jwtOwner, appID, "DATABASE_URL", realDB)
	putEnv(t, ts, jwtOwner, appID, "API_KEY", realKey)

	t.Run("list masks plaintext", func(t *testing.T) {
		runListMasksPlaintext(t, ts, jwtOwner, appID, realDB, realKey)
	})
	t.Run("owner reveal succeeds and audited as revealed", func(t *testing.T) {
		runOwnerReveal(t, pool, ts, jwtOwner, appID, realDB)
	})
	t.Run("member role lacks secret:reveal capability", func(t *testing.T) {
		runMemberForbidden(t, pool, ts, jwtOwner, appID)
	})
}

func runListMasksPlaintext(
	t *testing.T, ts *httptest.Server, jwtOwner, appID, realDB, realKey string,
) {
	code, body := authedJSON(t, http.MethodGet,
		ts.URL+"/v1/apps/"+appID+"/secrets", jwtOwner, nil)
	if code != http.StatusOK {
		t.Fatalf("list = %d", code)
	}
	if strings.Count(string(body), `"configured"`) < 2 {
		t.Fatalf("expected two configured values: %s", body)
	}
	if strings.Contains(string(body), realDB) || strings.Contains(string(body), realKey) {
		t.Fatal("plaintext leaked through listing")
	}
}

func runOwnerReveal(
	t *testing.T, pool *pgxpool.Pool, ts *httptest.Server,
	jwtOwner, appID, realDB string,
) {
	code, body := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/secrets/reveal", jwtOwner,
		map[string]string{"name": "DATABASE_URL"})
	if code != http.StatusOK {
		t.Fatalf("reveal = %d body=%s", code, body)
	}
	var out struct {
		Value string `json:"value"`
	}
	if decodeErr := json.Unmarshal(body, &out); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if out.Value != realDB {
		t.Fatalf("revealed %q, want %q", out.Value, realDB)
	}
	var result string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT result FROM audit_logs WHERE action='secret.reveal'
		 ORDER BY occurred_at DESC LIMIT 1`).Scan(&result); scanErr != nil {
		t.Fatal(scanErr)
	}
	if result != "revealed" {
		t.Fatalf("audit result = %q, want revealed", result)
	}
}

func runMemberForbidden(t *testing.T, pool *pgxpool.Pool, ts *httptest.Server, jwtOwner, appID string) {
	jwtMember := registerVerifiedUser(t, ts, "t19member")
	tm := NewTokenManager("test-secret-key-32-bytes-long!!")
	memberID, memberParseErr := tm.Parse(jwtMember)
	if memberParseErr != nil {
		t.Fatal(memberParseErr)
	}
	ownerID, ownerParseErr := tm.Parse(jwtOwner)
	if ownerParseErr != nil {
		t.Fatal(ownerParseErr)
	}
	store := &Store{pool: pool}
	orgA, orgErr := store.DefaultOrgForUser(context.Background(), ownerID)
	if orgErr != nil {
		t.Fatal(orgErr)
	}
	if _, execErr := pool.Exec(context.Background(),
		`INSERT INTO organization_members (user_id, org_id, role)
		 VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`,
		memberID, orgA); execErr != nil {
		t.Fatal(execErr)
	}

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/secrets/reveal",
		strings.NewReader(`{"name":"DATABASE_URL"}`))
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+jwtMember)
	req.Header.Set("X-Tenara-Org", orgA)
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("member reveal = %d, want 403", res.StatusCode)
	}
}
