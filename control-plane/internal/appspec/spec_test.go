package appspec

import (
	"errors"
	"strings"
	"testing"
)

const rbExample = `{
  "version": "v1",
  "services": {
    "web": {"type": "frontend", "runtime": "node", "path": "apps/web"},
    "api": {"type": "backend", "runtime": "python", "path": "apps/api", "port": 8000}
  },
  "database": {"mongodb": true},
  "routing": {
    "/": {"service": "web"},
    "/api": {"service": "api"}
  }
}`

func TestParseTableDriven(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		errFragment string
		wantErr     bool
	}{
		{name: "rb10 example passes", body: rbExample},
		{
			name:        "version v2 rejected",
			wantErr:     true,
			errFragment: "unsupported version",
			body:        strings.Replace(rbExample, `"v1"`, `"v2"`, 1),
		},
		{
			name:    "image field rejected",
			wantErr: true,
			body:    `{"version":"v1","services":{"api":{"type":"backend","runtime":"go","path":".","image":"repo/app:latest"}}}`,
		},
		{
			name:        "dangling routing rejected",
			wantErr:     true,
			errFragment: "unknown service",
			body:        strings.Replace(rbExample, `"service": "web"`, `"service": "ghost"`, 1),
		},
		{
			name:        "empty services rejected",
			wantErr:     true,
			errFragment: "at least one service",
			body:        `{"version":"v1","services":{}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, parseErr := Parse([]byte(tc.body))
			if !tc.wantErr {
				if parseErr != nil {
					t.Fatalf("unexpected error: %v", parseErr)
				}
				return
			}
			if parseErr == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(parseErr, ErrInvalidSpec) {
				t.Fatalf("not ErrInvalidSpec: %v", parseErr)
			}
			if tc.errFragment != "" && !strings.Contains(parseErr.Error(), tc.errFragment) {
				t.Fatalf("error %v lacks %q", parseErr, tc.errFragment)
			}
		})
	}
}

func TestWorkerAndCronServiceTypes(t *testing.T) {
	base := func(typ, extra string) string {
		return `{"version":"v1","services":{"svc":{"type":"` + typ +
			`","runtime":"node"` + extra + `}}}`
	}
	if _, pErr := Parse([]byte(base(TypeWorker, ""))); pErr != nil {
		t.Fatalf("worker without path must pass: %v", pErr)
	}
	if _, pErr := Parse([]byte(base(TypeCron, `,"schedule":"*/5 * * * *"`))); pErr != nil {
		t.Fatalf("cron good schedule: %v", pErr)
	}
	for _, bad := range []string{
		base(TypeCron, `,"schedule":"61 * * * *"`),
		base(TypeCron, `,"schedule":"* * * *"`),
		base(TypeCron, ""),
	} {
		if _, pErr := Parse([]byte(bad)); pErr == nil {
			t.Fatalf("must reject: %s", bad)
		}
	}
	if _, pErr := Parse([]byte(base(TypeBackend, `,"path":"/api","port":8080`))); pErr != nil {
		t.Fatalf("web-kind regression: %v", pErr)
	}
	routed := `{"version":"v1","services":{` +
		`"w":{"type":"worker","runtime":"go"},` +
		`"f":{"type":"frontend","runtime":"node","path":"/"}},` +
		`"routing":{"r":{"service":"w"}}}`
	if _, pErr := Parse([]byte(routed)); pErr == nil {
		t.Fatal("routing to worker must fail")
	}
}

func TestValidateSchedule(t *testing.T) {
	for _, okExpr := range []string{
		"* * * * *", "0 9 * * 1-5", "30 2 1,15 * *", "*/10 * * * *", "0 0 */2 * 0",
	} {
		if err := ValidateSchedule(okExpr); err != nil {
			t.Fatalf("%q: %v", okExpr, err)
		}
	}
	for _, badExpr := range []string{
		"* * * *", "* * * * * *", "60 * * * *", "* 24 * * *",
		"a * * * *", "1-0 * * * *", "*/0 * * * *", "5-40 * * * 8",
	} {
		if err := ValidateSchedule(badExpr); err == nil {
			t.Fatalf("%q must fail", badExpr)
		}
	}
}
