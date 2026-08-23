// Package boundary implements the R7 service-boundary algorithm: expand
// workspace globs, keep directories that carry a runtime entry signal, and
// exclude library-convention subtrees (plan tenara-agent-paas#24, R7).
package boundary

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrZeroServices = errors.New("zero services detected")

type ServiceCandidate struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Result struct {
	Services []ServiceCandidate `json:"services"`
}

var libDirNames = map[string]bool{
	"packages":  true,
	"libraries": true,
	"libs":      true,
	"common":    true,
	"shared":    true,
}

var skipDirNames = map[string]bool{
	"node_modules": true,
	".git":         true,
}

type pkgManifest struct {
	Scripts map[string]string `json:"scripts"`
	Name    string            `json:"name"`
	Main    string            `json:"main"`
}

func hasEntrySignal(pkgPath string) bool {
	content, readErr := os.ReadFile(pkgPath)
	if readErr != nil {
		return false
	}
	var m pkgManifest
	if json.Unmarshal(content, &m) != nil {
		return false
	}
	if strings.TrimSpace(m.Main) != "" {
		return true
	}
	for _, s := range []string{"dev", "start", "build"} {
		if _, ok := m.Scripts[s]; ok {
			return true
		}
	}
	return false
}

func readPkgName(pkgPath, fallback string) string {
	content, readErr := os.ReadFile(pkgPath)
	if readErr != nil {
		return fallback
	}
	var m struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(content, &m) != nil || m.Name == "" {
		return fallback
	}
	return m.Name
}

func hasGoSources(dir string) bool {
	matches, globErr := filepath.Glob(filepath.Join(dir, "*.go"))
	return globErr == nil && len(matches) > 0
}

func pyProjectHasScripts(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, "[project.scripts]") ||
		strings.Contains(s, "[tool.poetry.scripts]")
}

// parsePnpmGlobs extracts workspace globs without a YAML dependency.
func parsePnpmGlobs(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		return nil
	}
	var globs []string
	inList := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !inList {
			inList = strings.HasPrefix(trimmed, "packages:")
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		item := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")), "\"'")
		if item != "" {
			globs = append(globs, item)
		}
	}
	return globs
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// candidateName resolves the service name when dir qualifies under R7.
func candidateName(dir string) (string, bool) {
	pkgJSON := filepath.Join(dir, "package.json")
	goMod := filepath.Join(dir, "go.mod")
	pyProject := filepath.Join(dir, "pyproject.toml")

	switch {
	case fileExists(pkgJSON) && hasEntrySignal(pkgJSON):
		return readPkgName(pkgJSON, filepath.Base(dir)), true
	case fileExists(goMod) && hasGoSources(dir):
		return filepath.Base(dir), true
	case fileExists(pyProject) && pyProjectHasScripts(dir):
		return filepath.Base(dir), true
	default:
		return "", false
	}
}

// considerDir appends dir when it sits outside library-convention subtrees.
func considerDir(
	root, dir string, services *[]ServiceCandidate, seen map[string]bool,
) {
	clean := filepath.Clean(dir)
	rel, relErr := filepath.Rel(root, clean)
	if relErr != nil || rel == "." || seen[clean] {
		return
	}
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if libDirNames[seg] || skipDirNames[seg] {
			return
		}
	}
	name, ok := candidateName(clean)
	if !ok {
		return
	}
	seen[clean] = true
	*services = append(*services, ServiceCandidate{Name: name, Path: rel})
}

func appendRootService(
	root string, services *[]ServiceCandidate, seen map[string]bool,
) {
	pkgJSON := filepath.Join(root, "package.json")
	if !fileExists(pkgJSON) || !hasEntrySignal(pkgJSON) {
		return
	}
	if seen[root] {
		return
	}
	seen[root] = true
	name := readPkgName(pkgJSON, filepath.Base(root))
	*services = append(*services, ServiceCandidate{Name: name, Path: "."})
}

// Detect applies R7 over root. Paths are reported relative to root.
func Detect(root string) (*Result, error) {
	root = filepath.Clean(root)
	globs := parsePnpmGlobs(root)

	services := []ServiceCandidate{}
	seen := map[string]bool{}

	if len(globs) == 0 {
		appendRootService(root, &services, seen)
	} else {
		for _, g := range globs {
			matches, globErr := filepath.Glob(
				filepath.Join(root, filepath.FromSlash(g)))
			if globErr != nil {
				return nil, fmt.Errorf("glob %q: %w", g, globErr)
			}
			sort.Strings(matches)
			for _, m := range matches {
				if info, statErr := os.Stat(m); statErr == nil && info.IsDir() {
					considerDir(root, m, &services, seen)
				}
			}
		}
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Path < services[j].Path
	})
	if len(services) == 0 {
		return nil, fmt.Errorf("%w: no runnable service found under %s",
			ErrZeroServices, root)
	}
	return &Result{Services: services}, nil
}
