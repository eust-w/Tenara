package verify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func passStep(id int) StepResult {
	return StepResult{ID: id, Name: StepName(id), Status: StatusPass}
}

func fullPassingSteps() []StepResult {
	steps := make([]StepResult, 0, 9)
	for _, id := range StepIDs() {
		steps = append(steps, passStep(id))
	}
	return steps
}

func TestStepCatalogCompleteNineOrdered(t *testing.T) {
	ids := StepIDs()
	if len(ids) != 9 {
		t.Fatalf("catalog size = %d, want 9", len(ids))
	}
	for i, id := range ids {
		if id != i+1 {
			t.Fatalf("catalog[%d] = %d", i, id)
		}
		if StepName(id) == "" {
			t.Fatalf("step %d lacks a name", id)
		}
	}
	for _, id := range []int{7, 8, 9} {
		if !IsBrowserStep(id) {
			t.Fatalf("step %d must be flagged as browser", id)
		}
	}
	if IsBrowserStep(StepAPIHealth) || IsBrowserStep(10) || IsBrowserStep(0) {
		t.Fatal("browser-step boundaries wrong")
	}
}

func TestAllPassAggregatesRunning(t *testing.T) {
	at := time.Unix(1700000000, 0)
	rep := Aggregate("acme", fullPassingSteps(), at)
	if rep.Outcome != OutcomeRunning {
		t.Fatalf("outcome = %s", rep.Outcome)
	}
	if err := ValidateComplete(rep.Steps); err != nil {
		t.Fatal(err)
	}
	blob, jErr := json.Marshal(rep)
	if jErr != nil {
		t.Fatal(jErr)
	}
	if !strings.Contains(string(blob), `"outcome":"RUNNING"`) {
		t.Fatalf("json missing outcome field: %s", blob)
	}
}

func TestAnyFailAggregatesDegradedAndLocatesStepSix(t *testing.T) {
	steps := fullPassingSteps()
	steps[5].Status = StatusFail
	steps[5].Detail = "health route returned 404"

	rep := Aggregate("acme", steps, time.Unix(0, 0))
	if rep.Outcome != OutcomeDegraded {
		t.Fatalf("outcome = %s", rep.Outcome)
	}
	if rep.Steps[5].ID != StepAPIHealth || rep.Steps[5].Status != StatusFail {
		t.Fatalf("step six not pinpointed: %+v", rep.Steps[5])
	}
}

func TestValidateCompleteRejectsMissingBrowserStep(t *testing.T) {
	filtered := make([]StepResult, 0, 8)
	for _, s := range fullPassingSteps() {
		if s.ID != StepBrowserLoad {
			filtered = append(filtered, s)
		}
	}
	err := ValidateComplete(filtered)
	if err == nil || !strings.Contains(err.Error(), "9") {
		t.Fatalf("dropping a browser step must be rejected, got %v", err)
	}
}

func TestValidateCompleteRejectsWrongOrder(t *testing.T) {
	steps := fullPassingSteps()
	steps[4], steps[5] = steps[5], steps[4]

	if err := ValidateComplete(steps); err == nil ||
		!strings.Contains(err.Error(), "position") {
		t.Fatalf("out-of-order catalog must be rejected, got %v", err)
	}
}

func TestStepTimeoutLocked(t *testing.T) {
	if StepTimeout != 120*1000000000 {
		t.Fatalf("per-step timeout = %s, want 120s", StepTimeout)
	}
}
