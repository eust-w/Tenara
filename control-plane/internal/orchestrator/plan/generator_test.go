package plan

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"tenara/control-plane/internal/appspec"
)

const canonicalSpec = `{
  "version": "v1",
  "services": {
    "web": {"type": "frontend", "runtime": "node", "path": "apps/web"},
    "api": {"type": "backend", "runtime": "python", "path": "apps/api", "port": 8000}
  },
  "database": {"mongodb": true}
}`

func TestPlanGenerator(t *testing.T) {
	spec, parseErr := appspec.Parse([]byte(canonicalSpec))
	if parseErr != nil {
		t.Fatalf("fixture spec invalid: %v", parseErr)
	}

	got, genErr := Generate(Input{
		AppID:      "12345678-9abc-def0-1234-56789abcdef0",
		Slug:       "myshop",
		Env:        "production",
		BaseDomain: "tenara.local",
		Spec:       spec,
		Now:        time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC),
		TTL:        24 * time.Hour,
	})
	if genErr != nil {
		t.Fatal(genErr)
	}

	goldenRaw, readErr := os.ReadFile("../../../../e2e/fixtures/golden/plan-expected.json")
	if readErr != nil {
		t.Fatalf("golden fixture missing: %v", readErr)
	}
	var goldenMap map[string]any
	if decodeErr := json.Unmarshal(goldenRaw, &goldenMap); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	gotRaw, _ := json.Marshal(got)
	var gotMap map[string]any
	if decodeErr := json.Unmarshal(gotRaw, &gotMap); decodeErr != nil {
		t.Fatal(decodeErr)
	}

	// jq-style field-by-field equality over every golden key.
	for field, want := range goldenMap {
		have, ok := gotMap[field]
		if !ok {
			t.Errorf("missing golden field %q", field)
			continue
		}
		if !reflect.DeepEqual(want, have) {
			t.Errorf("field %q = %v, want %v", field, have, want)
		}
	}
}

func TestEstimatesWithinFreeCeilings(t *testing.T) {
	spec, parseErr := appspec.Parse([]byte(canonicalSpec))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	got, genErr := Generate(Input{
		AppID: "12345678-9abc-def0-1234-56789abcdef0", Slug: "myshop",
		Env: "production", BaseDomain: "tenara.local", Spec: spec,
		Now: time.Now(), TTL: time.Hour,
	})
	if genErr != nil {
		t.Fatal(genErr)
	}
	const capCPU, capMem = 1000, 1024 // RB-29 free ceiling sanity bounds
	for _, svc := range got.Services {
		if svc.Resources.CPUMillicores <= 0 || svc.Resources.CPUMillicores > capCPU {
			t.Errorf("%s cpu %d outside (0,%d]", svc.Name, svc.Resources.CPUMillicores, capCPU)
		}
		if svc.Resources.MemoryMegabytes <= 0 || svc.Resources.MemoryMegabytes > capMem {
			t.Errorf("%s mem %d outside (0,%d]", svc.Name, svc.Resources.MemoryMegabytes, capMem)
		}
	}
}
