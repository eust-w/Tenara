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

func registerKnownEmail(t *testing.T, ts *httptest.Server, email string) string {
	t.Helper()
	if code := post(t, ts.URL+"/v1/auth/register", uniqueIP(),
		map[string]string{"email": email, "password": "hunter2abc"}); code != http.StatusCreated {
		t.Fatalf("register %s = %d", email, code)
	}
	token := latestVerifyToken(t)
	post(t, ts.URL+"/v1/auth/verify", "", map[string]string{"token": token})

	res, loginErr := http.Post(ts.URL+"/v1/auth/login", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"password":"hunter2abc"}`, email)))
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	defer res.Body.Close()
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&parsed); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	return parsed.AccessToken
}

func TestMemberManagement(t *testing.T) {
	ts := newTestServer(t)
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	stamp := timeNowUnix()
	adminJWT := registerKnownEmail(t, ts, fmt.Sprintf("t13-admin-%d@test.tenara", stamp))

	t.Run("workspace_admin invites absent email", func(t *testing.T) {
		runInviteAbsentEmail(t, ts, adminJWT, stamp)
	})
	t.Run("member lacks member:invite in that org", func(t *testing.T) {
		runMemberLacksInvite(t, ts, pool, adminJWT)
	})
	t.Run("switching to unknown workspace fails closed", func(t *testing.T) {
		runUnknownWorkspaceSwitch(t, ts)
	})
}

func runInviteAbsentEmail(t *testing.T, ts *httptest.Server, adminJWT string, stamp int64) {
	memberEmail := fmt.Sprintf("t13-invited-%d@test.tenara", stamp)
	code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/members", adminJWT,
		map[string]string{"email": memberEmail, "role": "member"})
	if code != http.StatusCreated {
		t.Fatalf("invite = %d body=%s", code, body)
	}
	listCode, listBody := authedJSON(t, http.MethodGet, ts.URL+"/v1/members", adminJWT, nil)
	if listCode != http.StatusOK {
		t.Fatalf("list = %d", listCode)
	}
	if !strings.Contains(string(listBody), memberEmail) ||
		!strings.Contains(string(listBody), `"role":"member"`) {
		t.Fatalf("invited member missing from list: %s", listBody)
	}
}

func runMemberLacksInvite(t *testing.T, ts *httptest.Server, pool *pgxpool.Pool, adminJWT string) {
	tm := NewTokenManager("test-secret-key-32-bytes-long!!")
	adminID, parseErr := tm.Parse(adminJWT)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	store := &Store{pool: pool}
	orgA, orgErr := store.DefaultOrgForUser(context.Background(), adminID)
	if orgErr != nil {
		t.Fatal(orgErr)
	}

	memberJWT := registerVerifiedUser(t, ts, "t13member")
	memberID, memberParseErr := tm.Parse(memberJWT)
	if memberParseErr != nil {
		t.Fatal(memberParseErr)
	}
	if _, execErr := pool.Exec(context.Background(),
		`INSERT INTO organization_members (user_id, org_id, role)
		 VALUES ($1, $2, 'member') ON CONFLICT (user_id, org_id) DO NOTHING`,
		memberID, orgA); execErr != nil {
		t.Fatal(execErr)
	}

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/v1/members", strings.NewReader(
			fmt.Sprintf(`{"email":"x-%d@test.tenara","role":"member"}`, timeNowUnix())))
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+memberJWT)
	req.Header.Set("X-Tenara-Org", orgA)
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("member invite = %d body=%s, want 403", res.StatusCode, body)
	}
}

func runUnknownWorkspaceSwitch(t *testing.T, ts *httptest.Server) {
	memberJWT := registerVerifiedUser(t, ts, "t13cross")
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/v1/members", nil)
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+memberJWT)
	req.Header.Set("X-Tenara-Org", "00000000-0000-0000-0000-00000000dead")
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("unknown-workspace switch must not succeed")
	}
}

func TestEnsurePlatformAdmin(t *testing.T) {
	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	store := &Store{pool: pool}
	email := fmt.Sprintf("admin-boot-%d@test.tenara", timeNowUnix())
	if err := store.EnsurePlatformAdmin(context.Background(), email); err != nil {
		t.Fatal(err)
	}
	var role string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT m.role FROM organization_members m JOIN users u ON u.id = m.user_id
		 WHERE u.email = $1`, email).Scan(&role); scanErr != nil {
		t.Fatal(scanErr)
	}
	if role != "platform_admin" {
		t.Fatalf("role = %s, want platform_admin", role)
	}
	if err := store.EnsurePlatformAdmin(context.Background(), email); err != nil {
		t.Fatalf("second bootstrap must be idempotent: %v", err)
	}
}
