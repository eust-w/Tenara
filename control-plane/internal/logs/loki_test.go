package logs

import (
	"strings"
	"testing"
)

func TestBuildURLRequiresAppScope(t *testing.T) {
	q := Query{Source: SourceApp}
	if _, err := q.BuildURL("http://loki"); err == nil {
		t.Fatal("empty app scope must refuse to build a query")
	}
}

func TestBuildURLContainsScopedSelector(t *testing.T) {
	q := Query{AppID: "acme", Source: SourceBuild, Limit: 42}
	target, buildErr := q.BuildURL("http://loki.test")
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	checks := []string{
		"/loki/api/v1/query_range?",
		"app_id%3D%22acme%22",
		"limit=42",
	}
	for _, want := range checks {
		if !strings.Contains(target, want) {
			t.Fatalf("url missing %q: %s", want, target)
		}
	}
}

func TestBuildURLRejectsUnknownSource(t *testing.T) {
	q := Query{AppID: "a", Source: Source("platform")}
	if _, err := q.BuildURL("http://loki"); err == nil {
		t.Fatal("unknown source must be rejected")
	}
}

func TestParseLokiResponseFlattensPairs(t *testing.T) {
	frame := `{"data":{"result":[{"values":[["1700000000000000000","A"],["1700000001000000000","B"]]}]}}`
	lines, err := ParseLokiResponse([]byte(frame))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if lines[0].Text != "A" || lines[1].Text != "B" {
		t.Fatalf("texts = %q,%q", lines[0].Text, lines[1].Text)
	}
}

func TestParseLokiResponseInvalidBody(t *testing.T) {
	if _, err := ParseLokiResponse([]byte("{broken")); err == nil {
		t.Fatal("invalid json must surface as an error")
	}
}
