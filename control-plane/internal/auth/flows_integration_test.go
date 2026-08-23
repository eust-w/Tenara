package auth

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tenara/control-plane/internal/pgstore"
)

const (
	testDatabaseURL = "postgres://tenara:tenara_pg_dev@127.0.0.1:15432/tenara?sslmode=disable"
	mailhogAPI      = "http://127.0.0.1:8025/api/v2/messages"
)

// uniqueIP derives a fresh per-run client IP so rate-limit windows never
// collide between test executions.
func uniqueIP() string {
	b := make([]byte, 3)
	if _, err := crand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("10.%d.%d.%d", b[0]%200+2, b[1], b[2])
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool, err := pgstore.NewPool(context.Background(), testDatabaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	svc := NewService(NewStore(pool), NewTokenManager("test-secret-key-32-bytes-long!!"), "http://localhost:8080")
	r := chi.NewRouter()
	svc.Mount(r)
	return httptest.NewServer(r)
}

func post(t *testing.T, url string, xff string, body any) int {
	t.Helper()
	raw, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	req, reqErr := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	req.Header.Set("Content-Type", "application/json")
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatal(doErr)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(res.Body)
		t.Logf("5xx body: %s", buf.String())
	}
	return res.StatusCode
}

func latestVerifyToken(t *testing.T) string {
	t.Helper()
	res, err := http.Get(mailhogAPI)
	if err != nil {
		t.Skipf("mailhog unavailable: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		Items []struct {
			Content struct {
				Body string `json:"Body"`
			} `json:"Content"`
		} `json:"items"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&payload); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	re := regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
	for i := range payload.Items { // newest first
		if m := re.FindStringSubmatch(payload.Items[i].Content.Body); m != nil &&
			strings.Contains(payload.Items[i].Content.Body, "auth/verify") {
			return m[1]
		}
	}
	t.Fatal("no verification link found in mailhog")
	return ""
}

func TestEmailAuthFlows(t *testing.T) {
	ts := newTestServer(t)

	runHappyFlow(t, ts)

	runFailureCases(ts, t)
	runRateLimitCase(t, ts)

	t.Run("password reset single-use token", func(t *testing.T) {
		email := fmt.Sprintf("reset-%d@test.tenara", timeNowUnix())
		if code := post(t, ts.URL+"/v1/auth/register", uniqueIP(),
			map[string]string{"email": email, "password": "hunter2abc"}); code != http.StatusCreated {
			t.Fatalf("register = %d", code)
		}
		token := latestVerifyToken(t)
		post(t, ts.URL+"/v1/auth/verify", "", map[string]string{"token": token})
		if code := post(t, ts.URL+"/v1/auth/request-password-reset", "",
			map[string]string{"email": email}); code != http.StatusAccepted {
			t.Fatalf("request-reset = %d", code)
		}
		resetToken := latestResetToken(t)
		newPassword := "fresh-pass-99"
		if code := post(t, ts.URL+"/v1/auth/reset-password", "",
			map[string]string{"token": resetToken, "new_password": newPassword}); code != http.StatusNoContent {
			t.Fatalf("reset = %d", code)
		}
		if code := post(t, ts.URL+"/v1/auth/login", "",
			map[string]string{"email": email, "password": newPassword}); code != http.StatusOK {
			t.Fatalf("login with new password = %d", code)
		}
		if code := post(t, ts.URL+"/v1/auth/reset-password", "",
			map[string]string{"token": resetToken, "new_password": "another-pass-7"}); code != http.StatusGone {
			t.Fatalf("replayed reset = %d, want 410", code)
		}
	})
}

func runHappyFlow(t *testing.T, ts *httptest.Server) {
	email := fmt.Sprintf("happy-%d@test.tenara", timeNowUnix())
	xff := uniqueIP()
	if code := post(t, ts.URL+"/v1/auth/register", xff,
		map[string]string{"email": email, "password": "hunter2abc"}); code != http.StatusCreated {
		t.Fatalf("register = %d", code)
	}
	token := latestVerifyToken(t)
	if code := post(t, ts.URL+"/v1/auth/verify", "",
		map[string]string{"token": token}); code != http.StatusNoContent {
		t.Fatalf("verify = %d", code)
	}
	loginRes, err := http.Post(ts.URL+"/v1/auth/login", "application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"password":"hunter2abc"}`, email)))
	if err != nil {
		t.Fatal(err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login = %d", loginRes.StatusCode)
	}
	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if decodeErr := json.NewDecoder(loginRes.Body).Decode(&tokens); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("missing tokens")
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	meRes, meErr := http.DefaultClient.Do(meReq)
	if meErr != nil {
		t.Fatal(meErr)
	}
	defer meRes.Body.Close()
	if meRes.StatusCode != http.StatusOK {
		t.Fatalf("/v1/me = %d", meRes.StatusCode)
	}
}

func runRateLimitCase(t *testing.T, ts *httptest.Server) {
	xff := uniqueIP()
	for i := range 6 { // window allows exactly 5; the 6th must be rejected
		code := post(t, ts.URL+"/v1/auth/register", xff, map[string]string{
			"email":    fmt.Sprintf("ratelimit-%d-%d@test.tenara", timeNowUnix(), i),
			"password": "hunter2abc",
		})
		if i < 5 && code != http.StatusCreated {
			t.Fatalf("signup %d = %d, want 201", i+1, code)
		}
		if i == 5 && code != http.StatusTooManyRequests {
			t.Fatalf("signup %d = %d, want 429", i+1, code)
		}
	}
}

func latestResetToken(t *testing.T) string {
	res, err := http.Get(mailhogAPI)
	if err != nil {
		t.Skipf("mailhog unavailable: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		Items []struct {
			Content struct {
				Body string `json:"Body"`
			} `json:"Content"`
		} `json:"items"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&payload); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	re := regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
	for i := range payload.Items { // newest first
		body := payload.Items[i].Content.Body
		if strings.Contains(body, "auth/reset-password") {
			if m := re.FindStringSubmatch(body); m != nil {
				return m[1]
			}
		}
	}
	t.Fatal("no reset link found in mailhog")
	return ""
}

func runFailureCases(ts *httptest.Server, t *testing.T) {
	cases := []struct {
		setup func(t *testing.T) (email, password string)
		name  string
		want  int
	}{
		{
			name: "unverified login is 403",
			setup: func(t *testing.T) (string, string) {
				email := fmt.Sprintf("unverified-%d@test.tenara", timeNowUnix())
				if code := post(t, ts.URL+"/v1/auth/register", uniqueIP(),
					map[string]string{"email": email, "password": "hunter2abc"}); code != http.StatusCreated {
					t.Fatalf("register = %d", code)
				}
				return email, "hunter2abc"
			},
			want: http.StatusForbidden,
		},
		{
			name: "wrong password is 401",
			setup: func(t *testing.T) (string, string) {
				email := fmt.Sprintf("wrongpw-%d@test.tenara", timeNowUnix())
				post(t, ts.URL+"/v1/auth/register", uniqueIP(),
					map[string]string{"email": email, "password": "hunter2abc"})
				token := latestVerifyToken(t)
				post(t, ts.URL+"/v1/auth/verify", "", map[string]string{"token": token})
				return email, "wrong-password"
			},
			want: http.StatusUnauthorized,
		},
		{
			name: "weak password register is 422",
			setup: func(t *testing.T) (string, string) {
				return fmt.Sprintf("weak-%d@test.tenara", timeNowUnix()), "short"
			},
			want: http.StatusUnprocessableEntity,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			email, password := tc.setup(t)
			var code int
			switch tc.want {
			case http.StatusForbidden, http.StatusUnauthorized:
				code = post(t, ts.URL+"/v1/auth/login", "", map[string]string{"email": email, "password": password})
			default:
				code = post(t, ts.URL+"/v1/auth/register", "", map[string]string{"email": email, "password": password})
			}
			if code != tc.want {
				t.Fatalf("%s = %d, want %d", tc.name, code, tc.want)
			}
		})
	}
}

func timeNowUnix() int64 {
	return time.Now().UnixNano()
}
