package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAllEndpoints501 pins the contract skeleton: every declared operation is
// routed and answers 501 with an RFC7807 problem until its wave implements it.
func TestAllEndpoints501(t *testing.T) {
	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/healthz", ""},
		{http.MethodGet, "/readyz", ""},
		{http.MethodGet, "/v1/me", ""},
		{http.MethodPost, "/v1/auth/register", "{}"},
		{http.MethodPost, "/v1/auth/verify", "{}"},
		{http.MethodPost, "/v1/auth/login", "{}"},
		{http.MethodPost, "/v1/auth/request-password-reset", "{}"},
		{http.MethodPost, "/v1/auth/reset-password", "{}"},
		{http.MethodGet, "/v1/github/start", ""},
		{http.MethodGet, "/v1/github/callback?code=c&state=s", ""},
		{http.MethodGet, "/v1/github/repos", ""},
		{http.MethodDelete, "/v1/github/binding", ""},
		{http.MethodGet, "/v1/apps", ""},
		{http.MethodPost, "/v1/apps", "{}"},
		{http.MethodGet, "/v1/apps/a1", ""},
		{http.MethodPatch, "/v1/apps/a1", "{}"},
		{http.MethodDelete, "/v1/apps/a1", ""},
		{http.MethodPost, "/v1/apps/a1/services", "{}"},
		{http.MethodPost, "/v1/apps/a1/environments", "{}"},
		{http.MethodPost, "/v1/analyze", "{}"},
		{http.MethodPut, "/v1/apps/a1/spec", "{}"},
		{http.MethodGet, "/v1/apps/a1/plan", ""},
		{http.MethodPost, "/v1/apps/a1/deployments", "{}"},
		{http.MethodGet, "/v1/apps/a1/deployments/d1", ""},
		{http.MethodPost, "/v1/apps/a1/rollback", "{}"},
		{http.MethodPost, "/v1/apps/a1/restart", ""},
		{http.MethodGet, "/v1/apps/a1/logs", ""},
		{http.MethodPut, "/v1/apps/a1/env", "{}"},
		{http.MethodGet, "/v1/apps/a1/secrets", ""},
		{http.MethodPost, "/v1/apps/a1/secrets/reveal", "{}"},
		{http.MethodGet, "/v1/apps/a1/domains", ""},
		{http.MethodPost, "/v1/apps/a1/domains", "{}"},
		{http.MethodPost, "/v1/apps/a1/domains/dm1/verify", ""},
		{http.MethodPost, "/v1/apps/a1/databases", "{}"},
		{http.MethodGet, "/v1/apps/a1/diagnostics", ""},
		{http.MethodGet, "/v1/tokens", ""},
		{http.MethodPost, "/v1/tokens", "{}"},
		{http.MethodDelete, "/v1/tokens/t1", ""},
		{http.MethodGet, "/v1/members", ""},
		{http.MethodPost, "/v1/members", "{}"},
		{http.MethodGet, "/v1/auditlogs", ""},
		{http.MethodGet, "/v1/admin/users", ""},
		{http.MethodPost, "/v1/admin/users/u1/suspend", "{}"},
		{http.MethodGet, "/v1/admin/apps", ""},
		{http.MethodPut, "/v1/admin/quota", "{}"},
		{http.MethodGet, "/v1/admin/security-events", ""},
	}

	for _, tc := range cases {
		req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotImplemented {
			t.Errorf("%s %s = %d, want 501", tc.method, tc.path, resp.StatusCode)
		}
	}
}
