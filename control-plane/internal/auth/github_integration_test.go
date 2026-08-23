package auth

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/kms"
	"tenara/control-plane/internal/pgstore"
)

// mockGitHub spins up fake token + API endpoints mimicking GitHub OAuth.
func mockGitHub(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("code") != "good-code" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":""}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"ghs_mocktoken123","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); !strings.Contains(auth, "ghs_mocktoken123") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"login":"octocat-tenara"}`))
	})
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); !strings.Contains(auth, "ghs_mocktoken123") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"full_name":"acme/widget","private":false,"default_branch":"main"}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newGithubHarness(t *testing.T) (*GithubHandlers, *httptest.Server, *int, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgstore.NewPool(context.Background(), testDatabaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	store := NewStore(pool)
	kmsStub, kmsErr := kms.NewStub(strings.Repeat("ab", 32))
	if kmsErr != nil {
		t.Fatal(kmsErr)
	}
	ghSrv, calls := mockGitHub(t)
	gh := &GitHubOAuth{
		Store: store, KMS: kmsStub,
		ClientID: "cid", ClientSecret: "csecret",
		AuthBaseURL: ghSrv.URL, APIBaseURL: ghSrv.URL,
	}
	h := &GithubHandlers{GitHub: gh, Tokens: NewTokenManager("test-secret-key-32-bytes-long!!")}
	tokens := NewTokenManager("test-secret-key-32-bytes-long!!")
	svc := NewService(store, tokens, "http://localhost:8080")
	r := chi.NewRouter()
	svc.Mount(r)
	h.Tokens = tokens
	h.Mount(r)
	return h, httptest.NewServer(r), calls, pool
}

func bearerGet(t *testing.T, srvURL, path, bearer string) int {
	req, err := http.NewRequest(http.MethodGet, srvURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	return res.StatusCode
}

func registerVerifiedUser(t *testing.T, ts *httptest.Server, tag string) string {
	email := fmt.Sprintf("gh-%s-%d@test.tenara", tag, timeNowUnix())
	if code := post(t, ts.URL+"/v1/auth/register", uniqueIP(),
		map[string]string{"email": email, "password": "hunter2abc"}); code != http.StatusCreated {
		t.Fatalf("register = %d", code)
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

func TestGithubBinding(t *testing.T) {
	harness, ts, _, pool := newGithubHarness(t)

	t.Run("state mismatch callback is 403", func(t *testing.T) {
		bearer := registerVerifiedUser(t, ts, "mismatch")
		code := bearerGet(t, ts.URL,
			"/v1/github/callback?code=good-code&state=forged-state", bearer)
		if code != http.StatusForbidden {
			t.Fatalf("callback = %d, want 403", code)
		}
	})

	t.Run("bind repos ciphertext-at-rest unbind", func(t *testing.T) {
		bearer := registerVerifiedUser(t, ts, "bind")
		userID := parseUserID(t, harness.Tokens, bearer)

		startReq, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/v1/github/start", nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		startReq.Header.Set("Authorization", "Bearer "+bearer)
		noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		startRes, startErr := noRedirect.Do(startReq)
		if startErr != nil {
			t.Fatal(startErr)
		}
		defer startRes.Body.Close()
		if startRes.StatusCode != http.StatusFound {
			t.Fatalf("start = %d, want 302", startRes.StatusCode)
		}
		loc := startRes.Header.Get("Location")
		state := extractState(t, loc)

		callbackCode := bearerGet(t, ts.URL,
			"/v1/github/callback?code=good-code&state="+url.QueryEscape(state), bearer)
		if callbackCode != http.StatusOK {
			t.Fatalf("callback = %d, want 200", callbackCode)
		}

		if reposCode := bearerGet(t, ts.URL, "/v1/github/repos?page=1", bearer); reposCode != http.StatusOK {
			t.Fatalf("repos after bind = %d, want 200", reposCode)
		}

		var sealed []byte
		scanErr := pool.QueryRow(context.Background(),
			`SELECT github_token_encrypted FROM users WHERE id = $1`, userID).Scan(&sealed)
		if scanErr != nil {
			t.Fatal(scanErr)
		}
		if strings.Contains(hex.EncodeToString(sealed), "ghp_") || len(sealed) == 0 {
			t.Fatal("github_token_encrypted must be non-empty ciphertext without plaintext prefix")
		}

		unbindReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/github/binding", nil)
		unbindReq.Header.Set("Authorization", "Bearer "+bearer)
		unbindRes, doErr := http.DefaultClient.Do(unbindReq)
		if doErr != nil {
			t.Fatal(doErr)
		}
		defer unbindRes.Body.Close()
		if unbindRes.StatusCode != http.StatusNoContent {
			t.Fatalf("unbind = %d, want 204", unbindRes.StatusCode)
		}

		if after := bearerGet(t, ts.URL, "/v1/github/repos?page=1", bearer); after != http.StatusUnauthorized {
			t.Fatalf("repos after unbind = %d, want 401", after)
		}
	})
}

var stateRe = regexp.MustCompile(`state=([A-Za-z0-9_-]+)`)

func extractState(t *testing.T, loc string) string {
	m := stateRe.FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("no state in %q", loc)
	}
	return m[1]
}

func parseUserID(t *testing.T, tm *TokenManager, bearer string) string {
	id, err := tm.Parse(bearer)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
