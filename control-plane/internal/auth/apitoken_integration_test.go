package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/pgstore"
)

func authedJSON(t *testing.T, method, url, bearer string, body any) (int, []byte) {
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
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	raw, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return res.StatusCode, raw
}

func TestOrgScopedAPITokens(t *testing.T) {
	ts := newTestServer(t)
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	jwt := registerVerifiedUser(t, ts, "t12")

	t.Run("create returns plaintext exactly once", func(t *testing.T) {
		runCreateOnce(t, ts, jwt, pool)
	})
	t.Run("token works on protected endpoint then revoked becomes 401", func(t *testing.T) {
		runRevokeFlow(t, ts, jwt)
	})
	t.Run("listing never leaks plaintext or hash", func(t *testing.T) {
		runLeakCheck(t, ts, jwt)
	})
	t.Run("tokens are scoped to their own org", func(t *testing.T) {
		runOrgScope(t, ts, pool, jwt)
	})
}

func runCreateOnce(t *testing.T, ts *httptest.Server, jwt string, pool *pgxpool.Pool) {
	code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/tokens", jwt,
		map[string]string{"name": "ci"})
	if code != http.StatusCreated {
		t.Fatalf("create = %d body=%s", code, body)
	}
	var created struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Prefix    string `json:"prefix"`
		Plaintext string `json:"plaintext"`
	}
	if decodeErr := json.Unmarshal(body, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	const p = "tenara_"
	if !strings.HasPrefix(created.Plaintext, p) {
		t.Fatalf("plaintext %q lacks prefix", created.Plaintext)
	}
	if got := created.Plaintext[:len(p)+apiTokenDisplayN]; created.Prefix != got {
		t.Fatalf("prefix %q want %q", created.Prefix, got)
	}

	sum := sha256.Sum256([]byte(created.Plaintext))
	wantHash := hex.EncodeToString(sum[:])
	var storedHash string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT token_hash FROM api_tokens WHERE id = $1`, created.ID).Scan(&storedHash); scanErr != nil {
		t.Fatal(scanErr)
	}
	if storedHash != wantHash {
		t.Fatal("stored hash != sha256(plaintext)")
	}
	var plainCols int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_name = 'api_tokens' AND column_name LIKE '%plain%'`).Scan(&plainCols); scanErr != nil {
		t.Fatal(scanErr)
	}
	if plainCols != 0 {
		t.Fatal("api_tokens must not have plaintext columns")
	}

	t.Cleanup(func() {
		_, _ = authedJSON(t, http.MethodDelete, ts.URL+"/v1/tokens/"+created.ID, jwt, nil)
	})
}

func runRevokeFlow(t *testing.T, ts *httptest.Server, jwt string) {
	_, createBody := authedJSON(t, http.MethodPost, ts.URL+"/v1/tokens", jwt,
		map[string]string{"name": "revokeme"})
	var created struct {
		ID        string `json:"id"`
		Plaintext string `json:"plaintext"`
	}
	if decodeErr := json.Unmarshal(createBody, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}

	code, meBody := authedJSON(t, http.MethodGet, ts.URL+"/v1/me", created.Plaintext, nil)
	if code != http.StatusOK {
		t.Fatalf("me with token = %d body=%s", code, meBody)
	}
	if !strings.Contains(string(meBody), `"org_id":"`) || strings.Contains(string(meBody), `"org_id":""`) {
		t.Fatalf("me missing org scope: %s", meBody)
	}

	delCode, delBody := authedJSON(t, http.MethodDelete,
		ts.URL+"/v1/tokens/"+created.ID, jwt, nil)
	if delCode != http.StatusNoContent {
		t.Fatalf("revoke = %d body=%s", delCode, delBody)
	}

	afterCode, _ := authedJSON(t, http.MethodGet, ts.URL+"/v1/me", created.Plaintext, nil)
	if afterCode != http.StatusUnauthorized {
		t.Fatalf("me after revoke = %d, want 401", afterCode)
	}
}

func runLeakCheck(t *testing.T, ts *httptest.Server, jwt string) {
	_, createBody := authedJSON(t, http.MethodPost, ts.URL+"/v1/tokens", jwt,
		map[string]string{"name": "leakcheck"})
	var created struct {
		ID        string `json:"id"`
		Plaintext string `json:"plaintext"`
	}
	if decodeErr := json.Unmarshal(createBody, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	defer func() {
		_, _ = authedJSON(t, http.MethodDelete, ts.URL+"/v1/tokens/"+created.ID, jwt, nil)
	}()

	listCode, listBody := authedJSON(t, http.MethodGet, ts.URL+"/v1/tokens", jwt, nil)
	if listCode != http.StatusOK {
		t.Fatalf("list = %d", listCode)
	}
	sum := sha256.Sum256([]byte(created.Plaintext))
	for _, secret := range []string{created.Plaintext, hex.EncodeToString(sum[:])} {
		if strings.Contains(string(listBody), secret) {
			t.Fatalf("list leaked %q", secret[:12]+"...")
		}
	}
}

func runOrgScope(t *testing.T, ts *httptest.Server, pool *pgxpool.Pool, jwt string) {
	_, bodyA := authedJSON(t, http.MethodPost, ts.URL+"/v1/tokens", jwt,
		map[string]string{"name": "orgA"})
	var tokA struct {
		Plaintext string `json:"plaintext"`
	}
	if decodeErr := json.Unmarshal(bodyA, &tokA); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	store := &Store{pool: pool}
	_, orgA, resolveErr := store.ResolveAPIToken(context.Background(), tokA.Plaintext)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if orgA == "" {
		t.Fatal("token resolved without an org scope")
	}

	jwt2 := registerVerifiedUser(t, ts, "t12b")
	tm := NewTokenManager("test-secret-key-32-bytes-long!!")
	userID2, parseErr := tm.Parse(jwt2)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	orgB, orgErr := store.DefaultOrgForUser(context.Background(), userID2)
	if orgErr != nil {
		t.Fatal(orgErr)
	}
	if orgB == "" || orgB == orgA {
		t.Fatalf("org isolation broken: A=%s B=%s", orgA, orgB)
	}
}
