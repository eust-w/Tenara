package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tenara/control-plane/internal/pgstore"
)

func TestAppsHTTPAcceptance(t *testing.T) {
	ts := newTestServer(t)
	jwtA := registerVerifiedUser(t, ts, "t16a")

	t.Run("fourth app returns 402 quota_exceeded and cross-org 404", func(t *testing.T) {
		runQuotaAndCrossOrg(t, ts, jwtA)
	})
	t.Run("environment lands row and duplicate conflicts", func(t *testing.T) {
		runEnvironmentFlow(t, ts)
	})
	t.Run("manual spec override accepts valid rejects invalid", func(t *testing.T) {
		runSpecOverride(t, ts)
	})
}

func runSpecOverride(t *testing.T, ts *httptest.Server) {
	jwtS := registerVerifiedUser(t, ts, "t17s")
	stamp := timeNowUnix()
	code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/apps", jwtS,
		map[string]string{"name": fmt.Sprintf("specapp-%d", stamp)})
	if code != http.StatusCreated {
		t.Fatalf("create = %d body=%s", code, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if decodeErr := json.Unmarshal(body, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}

	validSpec := `{"version":"v1","services":{"api":{"type":"backend","runtime":"python","path":"apps/api","port":8000}},"database":{"mongodb":true},"routing":{"/":{"service":"api"}}}`
	res, _ := authedRaw(t, http.MethodPut, ts.URL+"/v1/apps/"+created.ID+"/spec",
		jwtS, "", json.RawMessage(validSpec))
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("valid spec = %d, want 204", res.StatusCode)
	}

	v2Spec := strings.Replace(validSpec, `"v1"`, `"v2"`, 1)
	resV2, bodyV2 := authedRaw(t, http.MethodPut, ts.URL+"/v1/apps/"+created.ID+"/spec",
		jwtS, "", json.RawMessage(v2Spec))
	if resV2.StatusCode != http.StatusUnprocessableEntity ||
		!strings.Contains(string(bodyV2), "INVALID_SPEC") {
		t.Fatalf("v2 = %d body=%s, want 422 INVALID_SPEC", resV2.StatusCode, bodyV2)
	}

	imageSpec := `{"version":"v1","services":{"api":{"type":"backend","runtime":"go","path":".","image":"repo:latest"}}}`
	resImg, _ := authedRaw(t, http.MethodPut, ts.URL+"/v1/apps/"+created.ID+"/spec",
		jwtS, "", json.RawMessage(imageSpec))
	if resImg.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("image spec = %d, want 422", resImg.StatusCode)
	}
}

func runQuotaAndCrossOrg(t *testing.T, ts *httptest.Server, jwtA string) {
	stamp := timeNowUnix()
	var lastID string
	for i := range 3 {
		code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/apps", jwtA,
			map[string]string{"name": fmt.Sprintf("shop%d-%d", stamp, i)})
		if code != http.StatusCreated {
			t.Fatalf("create %d = %d body=%s", i+1, code, body)
		}
		var created struct {
			ID string `json:"id"`
		}
		if decodeErr := json.Unmarshal(body, &created); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		lastID = created.ID
	}
	code4, body4 := authedJSON(t, http.MethodPost, ts.URL+"/v1/apps", jwtA,
		map[string]string{"name": fmt.Sprintf("over-%d", stamp)})
	if code4 != http.StatusPaymentRequired {
		t.Fatalf("4th = %d body=%s, want 402", code4, body4)
	}
	if !strings.Contains(string(body4), "QUOTA_EXCEEDED") {
		t.Fatalf("missing error code: %s", body4)
	}
	jwtB := registerVerifiedUser(t, ts, "t16b")
	codeB, _ := authedJSON(t, http.MethodGet, ts.URL+"/v1/apps/"+lastID, jwtB, nil)
	if codeB != http.StatusNotFound {
		t.Fatalf("cross-org get = %d, want 404", codeB)
	}
}

func runEnvironmentFlow(t *testing.T, ts *httptest.Server) {
	jwtC := registerVerifiedUser(t, ts, "t16c")
	stamp := timeNowUnix()
	code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/apps", jwtC,
		map[string]string{"name": fmt.Sprintf("envapp-%d", stamp)})
	if code != http.StatusCreated {
		t.Fatalf("create = %d body=%s", code, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if decodeErr := json.Unmarshal(body, &created); decodeErr != nil {
		t.Fatal(decodeErr)
	}

	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	envCode, _ := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+created.ID+"/environments", jwtC,
		map[string]string{"name": "staging"})
	if envCode != http.StatusCreated {
		t.Fatalf("env create = %d, want 201", envCode)
	}
	var rowCount int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM environments WHERE app_id = $1 AND name = 'staging'`,
		created.ID).Scan(&rowCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if rowCount != 1 {
		t.Fatal("environment row missing")
	}
	dupCode, _ := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+created.ID+"/environments", jwtC,
		map[string]string{"name": "staging"})
	if dupCode != http.StatusConflict {
		t.Fatalf("duplicate env = %d, want 409", dupCode)
	}
}
