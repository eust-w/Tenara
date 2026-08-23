package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bannedImplFragments lists concrete provider implementation markers that may
// never appear among core dependencies (RB§40 red line); providers/types
// stays allowed as the sole consumption surface.
var bannedImplFragments = []string{
	"tenara/providers/mongo",
	"tenara/providers/redis",
	"tenara/providers/cache",
	"tenara/providers/storage",
	"tenara/providers/registry/local",
	"tenara/providers/secret",
	"providers/baidu",
}

func TestCoreDoesNotImportProviders(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("tenara/providers")) {
		return // not wired yet: nothing can leak today
	}
	out, lerr := exec.Command("go", "list", "-deps", "./...").CombinedOutput()
	if lerr != nil {
		t.Fatalf("go list: %v\n%s", lerr, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, banned := range bannedImplFragments {
			if strings.Contains(line, banned) {
				t.Errorf("core depends on concrete provider implementation %s", line)
			}
		}
	}
}
