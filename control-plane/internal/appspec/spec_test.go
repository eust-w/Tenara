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
