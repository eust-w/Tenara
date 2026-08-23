package boundary

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if mkdirErr := os.MkdirAll(filepath.Dir(full), 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(full, []byte(content), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
}

func TestMonorepoExcludesLibraries(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")
	writeFixture(t, root, "apps/web/package.json",
		`{"name":"web","scripts":{"dev":"next dev"}}`)
	writeFixture(t, root, "apps/api/pyproject.toml",
		"[project.scripts]\napi = \"api.main:app\"\n")
	writeFixture(t, root, "packages/ui/package.json", `{"name":"ui"}`)

	res, detectErr := Detect(root)
	if detectErr != nil {
		t.Fatal(detectErr)
	}
	if len(res.Services) != 2 {
		t.Fatalf("services = %+v, want 2 (ui excluded)", res.Services)
	}
	var gotPaths []string
	if len(res.Services) > 0 {
		gotPaths = make([]string, 0, len(res.Services))
	}
	for _, s := range res.Services {
		if strings.HasPrefix(s.Path, "packages/") {
			t.Fatalf("library subtree leaked: %s", s.Path)
		}
		gotPaths = append(gotPaths, s.Path)
	}
	if gotPaths[0] != filepath.Join("apps", "api") ||
		gotPaths[1] != filepath.Join("apps", "web") {
		t.Fatalf("paths = %v, want [apps/api apps/web]", gotPaths)
	}
}

func TestSinglePackageRepo(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "package.json",
		`{"name":"solo","main":"index.js","scripts":{"start":"node index.js"}}`)

	res, detectErr := Detect(root)
	if detectErr != nil {
		t.Fatal(detectErr)
	}
	if len(res.Services) != 1 ||
		res.Services[0].Path != "." || res.Services[0].Name != "solo" {
		t.Fatalf("services = %+v, want single '.' solo", res.Services)
	}
}

func TestEmptyWorkspaceZeroServices(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")

	_, detectErr := Detect(root)
	if !errors.Is(detectErr, ErrZeroServices) {
		t.Fatalf("err = %v, want ErrZeroServices", detectErr)
	}
}

func TestNodeModulesNeverCandidate(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "node_modules/leftover/package.json",
		`{"name":"leftover","main":"x.js","scripts":{"build":"b"}}`)
	writeFixture(t, root, "package.json", `{"name":"root-no-entry"}`)

	_, detectErr := Detect(root)
	if !errors.Is(detectErr, ErrZeroServices) {
		t.Fatalf("node_modules leaked as candidate: %v", detectErr)
	}
}
