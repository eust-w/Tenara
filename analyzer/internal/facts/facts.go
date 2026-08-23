// Package facts builds the raw file-detection matrix over the RB-10 scan
// list. Layer discipline: presence and shape only; no framework judgment.
package facts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileFact is one matched scan-list entry with a deterministic summary.
type FileFact struct {
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`
	SHA256  string   `json:"sha256"`
	Lines   int      `json:"lines"`
	TopKeys []string `json:"top_keys,omitempty"`
}

// RawFacts is the stage-facts document.
type RawFacts struct {
	Schema string     `json:"schema"`
	Root   string     `json:"root"`
	Files  []FileFact `json:"files"`
}

var exactKinds = map[string]string{
	"package.json":        "package-json",
	"pnpm-workspace.yaml": "pnpm-workspace",
	"turbo.json":          "turbo-config",
	"Dockerfile":          "dockerfile",
	"docker-compose.yml":  "docker-compose",
	"requirements.txt":    "requirements-txt",
	"pyproject.toml":      "pyproject-toml",
	"go.mod":              "go-mod",
	"Cargo.toml":          "cargo-toml",
	".env.example":        "env-example",
}

var prefixKinds = map[string]string{
	"next.config.": "next-config",
	"vite.config.": "vite-config",
	"nuxt.config.": "nuxt-config",
}

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".next":        true,
	"dist":         true,
	".turbo":       true,
	"vendor":       true,
}

func kindFor(base string) (string, bool) {
	if k, ok := exactKinds[base]; ok {
		return k, true
	}
	for prefix, k := range prefixKinds {
		if strings.HasPrefix(base, prefix) {
			return k, true
		}
	}
	return "", false
}

func countLines(b []byte) int { return bytes.Count(b, []byte("\n")) }

// Build walks root and returns the detection matrix; output ordering is
// fully deterministic (sorted paths, sorted top-level JSON keys).
func Build(root string) (*RawFacts, error) {
	facts := &RawFacts{Schema: "facts.v1", Root: root, Files: []FileFact{}}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if skipDirs[name] && path != root {
				return fs.SkipDir
			}
			return nil
		}
		kind, ok := kindFor(name)
		if !ok {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		sum := sha256.Sum256(content)
		fact := FileFact{
			Path:   path,
			Kind:   kind,
			SHA256: hex.EncodeToString(sum[:])[:12],
			Lines:  countLines(content),
		}
		if strings.HasSuffix(name, ".json") {
			var obj map[string]any
			if json.Unmarshal(content, &obj) == nil {
				for key := range obj {
					fact.TopKeys = append(fact.TopKeys, key)
				}
				sort.Strings(fact.TopKeys)
			}
		}
		facts.Files = append(facts.Files, fact)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(facts.Files, func(i, j int) bool {
		return facts.Files[i].Path < facts.Files[j].Path
	})
	return facts, nil
}
