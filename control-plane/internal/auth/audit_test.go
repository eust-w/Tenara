package auth

import (
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

func TestAuditTrail(t *testing.T) {
	ts := newTestServer(t)
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	jwt := registerVerifiedUser(t, ts, "t15")

	t.Run("agent UA attributed and no secret leak", func(t *testing.T) {
		runAgentAttribution(t, ts, pool, jwt)
	})
	t.Run("anonymous failure raises security event", func(t *testing.T) {
		runAnonymousSecurityEvent(t, ts, pool)
	})
	t.Run("auditlogs endpoint scoped to workspace", func(t *testing.T) {
		runAuditLogsEndpoint(t, ts, jwt)
	})
}

func runAgentAttribution(t *testing.T, ts *httptest.Server, pool *pgxpool.Pool, jwt string) {
	stamp := timeNowUnix()
	key, keyErr := RandomToken()
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/v1/tokens",
		strings.NewReader(fmt.Sprintf(`{"name":"audit-a-%d"}`, stamp)))
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("User-Agent", "codex/1.0")
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	raw, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d body=%s", res.StatusCode, raw)
	}

	var actorType, agent string
	var afterNull bool
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT actor_type, COALESCE(agent,''), "after" IS NULL
		 FROM audit_logs WHERE action = 'api_token.create'
		 ORDER BY occurred_at DESC LIMIT 1`).Scan(&actorType, &agent, &afterNull); scanErr != nil {
		t.Fatal(scanErr)
	}
	if actorType != "agent" || agent != "codex" {
		t.Fatalf("attribution = %s/%s, want agent/codex", actorType, agent)
	}
	if !afterNull {
		t.Fatal("after snapshot must stay empty for token creation")
	}

	var created struct {
		Plaintext string `json:"plaintext"`
	}
	if decodeErr := json.Unmarshal(raw, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	var leaks int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE after::text LIKE $1`,
		"%"+created.Plaintext+"%").Scan(&leaks); scanErr != nil {
		t.Fatal(scanErr)
	}
	if leaks != 0 {
		t.Fatal("plaintext leaked into audit_logs")
	}
}

func runAnonymousSecurityEvent(t *testing.T, ts *httptest.Server, pool *pgxpool.Pool) {
	var before int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM security_events`).Scan(&before); scanErr != nil {
		t.Fatal(scanErr)
	}
	code, _ := authedJSON(t, http.MethodPost, ts.URL+"/v1/members", "",
		map[string]string{
			"email": fmt.Sprintf("anon-%d@test.tenara", timeNowUnix()),
			"role":  "member",
		})
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", code)
	}
	var after int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM security_events`).Scan(&after); scanErr != nil {
		t.Fatal(scanErr)
	}
	if after < before+1 {
		t.Fatalf("security_events %d -> %d, want increase", before, after)
	}
}

func runAuditLogsEndpoint(t *testing.T, ts *httptest.Server, jwt string) {
	code, body := authedJSON(t, http.MethodGet, ts.URL+"/v1/auditlogs", jwt, nil)
	if code != http.StatusOK {
		t.Fatalf("auditlogs = %d body=%s", code, body)
	}
	if !strings.Contains(string(body), "api_token.create") {
		t.Fatalf("expected api_token.create row: %s", body)
	}
}
