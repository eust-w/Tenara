package diagnose

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tenara/verifier/internal/verify"
)

var at = time.Unix(1700000000, 0)

func degradedWithStep6() *verify.Report {
	rep := &verify.Report{AppID: "acme", Outcome: verify.OutcomeDegraded}
	for _, id := range verify.StepIDs() {
		st := verify.StatusPass
		if id == verify.StepAPIHealth {
			st = verify.StatusFail
		}
		rep.Steps = append(rep.Steps, verify.StepResult{
			ID: id, Name: verify.StepName(id), Status: st, Detail: "health route 404",
		})
	}
	return rep
}

func TestClassifiesMissingDatabaseURL(t *testing.T) {
	d := Classify(Bundle{
		GeneratedAt:   at,
		AppID:         "acme",
		VerifyReport:  degradedWithStep6(),
		ContainerLogs: "panic: required env DATABASE_URL is not set\nexit status 1",
	})
	if d.Classified != ClassMissingEnvDatabaseURL {
		t.Fatalf("class = %s", d.Classified)
	}
	if !strings.Contains(d.Hint, "mongo") {
		t.Fatalf("hint must suggest binding mongo: %q", d.Hint)
	}
	want := []int{verify.StepAPIHealth}
	if len(d.FailedSteps) != 1 || d.FailedSteps[0] != want[0] {
		t.Fatalf("failed steps = %v", d.FailedSteps)
	}
}

func TestClassifiesOOMKilled(t *testing.T) {
	d := Classify(Bundle{
		GeneratedAt: at,
		AppID:       "acme",
		PodEvents:   `Reason: OOMKilled  Message: container exceeded memory limit`,
	})
	if d.Classified != ClassOOMKilled {
		t.Fatalf("class = %s", d.Classified)
	}
}

func TestClassifiesCrashLoopAndImagePull(t *testing.T) {
	crash := Classify(Bundle{GeneratedAt: at, PodEvents: "Back-off restarting failed container (CrashLoopBackOff)"})
	if crash.Classified != ClassCrashLoopBackOff {
		t.Fatalf("crash class = %s", crash.Classified)
	}
	pull := Classify(Bundle{GeneratedAt: at, PodEvents: "ErrImagePull: manifest unknown"})
	if pull.Classified != ClassImagePullBackOff {
		t.Fatalf("pull class = %s", pull.Classified)
	}
	gw := Classify(Bundle{GeneratedAt: at, BuildLogTail: "", PodEvents: "", ContainerLogs: "GET /api/health -> 502 Bad Gateway"})
	if gw.Classified != ClassUpstream502 {
		t.Fatalf("gateway class = %s", gw.Classified)
	}
}

func TestCollectsFailedVerifySteps(t *testing.T) {
	rep := degradedWithStep6()
	rep.Steps[0].Status = verify.StatusFail
	rep.Steps[1].Status = verify.StatusFail
	d := Classify(Bundle{GeneratedAt: at, AppID: "acme", VerifyReport: rep})
	if len(d.FailedSteps) != 3 {
		t.Fatalf("failed steps = %v, want [1 2 6]", d.FailedSteps)
	}
}

func TestUnclassifiedFallback(t *testing.T) {
	d := Classify(Bundle{GeneratedAt: at, AppID: "acme"})
	if d.Classified != ClassUnclassified || d.Hint != "" {
		t.Fatalf("empty bundle must fall back cleanly: %+v", d)
	}
}

func TestDiagnosisNeverCarriesRawLogsOrSecrets(t *testing.T) {
	d := Classify(Bundle{
		GeneratedAt:   at,
		AppID:         "acme",
		ContainerLogs: "connecting mongodb://root:sup3rsecret@db/admin — crashed",
		PodEvents:     "OOMKilled",
	})
	blob, mErr := json.Marshal(d)
	if mErr != nil {
		t.Fatal(mErr)
	}
	for _, banned := range []string{"sup3rsecret", "mongodb://", "crashed"} {
		if strings.Contains(string(blob), banned) {
			t.Fatalf("diagnosis leaked %q: %s", banned, blob)
		}
	}
	if d.Classified != ClassOOMKilled {
		t.Fatalf("class = %s", d.Classified)
	}
}
