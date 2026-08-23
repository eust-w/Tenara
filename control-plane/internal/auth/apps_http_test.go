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
	t.Run("plan approval flow with expiry", func(t *testing.T) {
		runPlanFlow(t, ts)
	})
}

func runPlanFlow(t *testing.T, ts *httptest.Server) {
	jwtP := registerVerifiedUser(t, ts, "t18p")
	stamp := timeNowUnix()
	appID := createNamedApp(t, ts, jwtP, fmt.Sprintf("planned-%d", stamp))

	putCanonicalSpec(t, ts, jwtP, appID)
	plan1 := fetchPlan(t, ts, jwtP, appID)
	if !strings.HasPrefix(plan1.NamespaceName, "app-") || plan1.PlanID == "" ||
		!strings.HasSuffix(plan1.Domain, ".nip.io") {
		t.Fatalf("plan incomplete: %+v", plan1)
	}

	approvePlan(t, ts, jwtP, appID, plan1.PlanID, http.StatusAccepted, "PLANNED")

	pool, poolErr := pgstore.NewPool(context.Background(), testDatabaseURL)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)
	tm := NewTokenManager("test-secret-key-32-bytes-long!!")
	userID, parseErr := tm.Parse(jwtP)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	var auditCount int
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action='app.deploy' AND actor_id=$1`,
		userID).Scan(&auditCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if auditCount < 1 {
		t.Fatal("deploy audit row missing approver")
	}

	replayCode, _ := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/deployments", jwtP,
		map[string]string{"plan_id": plan1.PlanID})
	if replayCode != http.StatusConflict {
		t.Fatalf("replay approve = %d, want 409", replayCode)
	}

	plan2 := fetchPlan(t, ts, jwtP, appID)
	expireSnapshot(t, pool, plan2.PlanID)
	approvePlan(t, ts, jwtP, appID, plan2.PlanID, http.StatusGone, "PLAN_EXPIRED")
}

func createNamedApp(t *testing.T, ts *httptest.Server, jwt, name string) string {
	t.Helper()
	code, body := authedJSON(t, http.MethodPost, ts.URL+"/v1/apps", jwt,
		map[string]string{"name": name})
	if code != http.StatusCreated {
		t.Fatalf("create = %d body=%s", code, body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if decodeErr := json.Unmarshal(body, &out); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	return out.ID
}

func putCanonicalSpec(t *testing.T, ts *httptest.Server, jwt, appID string) {
	t.Helper()
	const spec = `{"version":"v1","services":{"api":{"type":"backend","runtime":"python","path":"apps/api","port":8000}},"database":{"mongodb":true}}`
	res, _ := authedRaw(t, http.MethodPut, ts.URL+"/v1/apps/"+appID+"/spec",
		jwt, "", json.RawMessage(spec))
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("spec put = %d", res.StatusCode)
	}
}

type planView struct {
	PlanID        string `json:"plan_id"`
	NamespaceName string `json:"namespace_name"`
	Domain        string `json:"domain"`
}

func fetchPlan(t *testing.T, ts *httptest.Server, jwt, appID string) planView {
	t.Helper()
	code, body := authedJSON(t, http.MethodGet,
		ts.URL+"/v1/apps/"+appID+"/plan", jwt, nil)
	if code != http.StatusOK {
		t.Fatalf("plan = %d body=%s", code, body)
	}
	var p planView
	if decodeErr := json.Unmarshal(body, &p); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	return p
}

func approvePlan(
	t *testing.T, ts *httptest.Server, jwt, appID, planID string,
	wantCode int, wantFragment string,
) {
	t.Helper()
	code, body := authedJSON(t, http.MethodPost,
		ts.URL+"/v1/apps/"+appID+"/deployments", jwt,
		map[string]string{"plan_id": planID})
	if code != wantCode || !strings.Contains(string(body), wantFragment) {
		t.Fatalf("deploy = %d body=%s, want %d %q", code, body, wantCode, wantFragment)
	}
}

func expireSnapshot(t *testing.T, pool *pgxpool.Pool, planID string) {
	t.Helper()
	if _, execErr := pool.Exec(context.Background(),
		`UPDATE deployments SET plan_snapshot = jsonb_set(plan_snapshot,'{expires_at}','"2020-01-01T00:00:00Z"') WHERE plan_id=$1`,
		planID); execErr != nil {
		t.Fatal(execErr)
	}
}

func runSpecOverride(t *testing.T, ts *httptest.Server) {
	t.Helper()
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
