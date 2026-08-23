package generator

import (
	"errors"
	"strings"
	"testing"

	"github.com/tenara/analyzer/internal/detectors"
)

func TestMixedRepoGeneratesCorrectSpec(t *testing.T) {
	services := map[string]detectors.ServiceFacts{
		"apps/web": {Dir: "apps/web", Framework: "next", Runtime: "node", Port: 3000},
		"apps/api": {Dir: "apps/api", Framework: "fastapi", Runtime: "python", Port: 8000},
	}
	mongo := detectors.MongoEvidence{Found: true, Evidence: "mongoose dependency"}

	res, genErr := Generate(services, mongo, "myshop", "tenara.local")
	if genErr != nil {
		t.Fatal(genErr)
	}

	if res.Spec.Version != "v1" {
		t.Fatal("version must be v1")
	}
	if len(res.Spec.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(res.Spec.Services))
	}

	assertService(t, res, "apps/web", "frontend", "node", 3000)
	assertService(t, res, "apps/api", "backend", "python", 8000)
	assertMongoTrue(t, res)
	assertRouting(t, res, "/", "apps/web")
	assertRouting(t, res, "/api", "apps/api")
	assertConfidence(t, res)
}

func assertService(
	t *testing.T, res *Result, path, svcType, runtime string, port int,
) {
	t.Helper()
	svc, ok := res.Spec.Services[path]
	if !ok {
		t.Fatalf("service %q missing", path)
	}
	if svc.Type != svcType || svc.Runtime != runtime || svc.Port != port {
		t.Fatalf("%s = %+v", path, svc)
	}
}

func assertMongoTrue(t *testing.T, res *Result) {
	t.Helper()
	if res.Spec.Database == nil || !res.Spec.Database.MongoDB {
		t.Fatal("database.mongodb must be true")
	}
}

func assertRouting(t *testing.T, res *Result, path, svc string) {
	t.Helper()
	rt, ok := res.Spec.Routing[path]
	if !ok || rt.Service != svc {
		t.Fatalf("routing %q missing or wrong", path)
	}
}

func assertConfidence(t *testing.T, res *Result) {
	t.Helper()
	if len(res.Confidence) == 0 {
		t.Fatal("confidence report empty")
	}
}

func TestDjangoFailsUnsupportedStack(t *testing.T) {
	services := map[string]detectors.ServiceFacts{
		"backend": {Dir: "backend", Framework: "", Runtime: ""},
	}
	mongo := detectors.MongoEvidence{}

	_, genErr := Generate(services, mongo, "x", "y")
	if genErr == nil {
		t.Fatal("expected error for unsupported stack")
	}
	if !errors.Is(genErr, ErrUnsupportedStack) {
		t.Fatalf("err = %v, want ErrUnsupportedStack", genErr)
	}
	for _, runtime := range []string{"node", "python", "go"} {
		if !strings.Contains(genErr.Error(), runtime) {
			t.Fatalf("error must list supported runtime %q: %v", runtime, genErr)
		}
	}
}
