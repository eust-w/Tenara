// Package detectors implements R6 deterministic framework/database/port
// detection over service directories and repository roots.
package detectors

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ServiceFacts carries R6 detection results for one service directory.
type ServiceFacts struct {
	Dir       string `json:"dir"`
	Framework string `json:"framework"`
	Runtime   string `json:"runtime"`
	Port      int    `json:"port"`
}

// MongoEvidence reports whether the R6 three-way check hit.
type MongoEvidence struct {
	Evidence string
	Found    bool
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readIfPresent(path string) (string, bool) {
	if !fileExists(path) {
		return "", false
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false
	}
	return string(data), true
}

func hasGlobMatch(pattern string) bool {
	matches, globErr := filepath.Glob(pattern)
	return globErr == nil && len(matches) > 0
}

func detectNodeFramework(dir, pkgRaw string) string {
	lower := strings.ToLower(pkgRaw)
	hasConfigPrefix := func(prefix string) bool {
		return hasGlobMatch(filepath.Join(dir, prefix+"*"))
	}
	switch {
	case strings.Contains(lower, `"next"`) || hasConfigPrefix("next.config."):
		return "next"
	case strings.Contains(lower, `"nuxt"`) || hasConfigPrefix("nuxt.config."):
		return "nuxt"
	case strings.Contains(lower, `"vue"`):
		return "vue"
	case strings.Contains(lower, `"react"`):
		return "react"
	case strings.Contains(lower, `"vite"`):
		return "vite"
	default:
		return ""
	}
}

func portFromDockerfile(dir string) int {
	dockerRaw, ok := readIfPresent(filepath.Join(dir, "Dockerfile"))
	if !ok {
		return 0
	}
	for _, line := range strings.Split(dockerRaw, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "EXPOSE") {
			port, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				return 0
			}
			return port
		}
	}
	return 0
}

// DetectService inspects one service directory for framework/runtime/port.
func DetectService(dir string) ServiceFacts {
	f := ServiceFacts{Dir: dir}

	pkgRaw, hasPkg := readIfPresent(filepath.Join(dir, "package.json"))
	hasGoMod := fileExists(filepath.Join(dir, "go.mod"))
	pyRaw, hasPy := "", false
	for _, name := range []string{"pyproject.toml", "requirements.txt"} {
		if raw, ok := readIfPresent(filepath.Join(dir, name)); ok {
			pyRaw, hasPy = raw, true
			break
		}
	}

	switch {
	case hasPkg:
		f.Runtime = "node"
		f.Framework = detectNodeFramework(dir, pkgRaw)
		if f.Framework == "next" {
			f.Port = defaultPorts["next"]
		}
	case hasPy && strings.Contains(strings.ToLower(pyRaw), "fastapi"):
		f.Runtime = "python"
		f.Framework = "fastapi"
		f.Port = defaultPorts["fastapi"]
	case hasGoMod:
		f.Runtime = "go"
		f.Port = 8080
	}

	if f.Port == 0 {
		f.Port = defaultPorts[f.Framework]
	}
	if expose := portFromDockerfile(dir); expose > 0 {
		f.Port = expose
	}
	return f
}

var defaultPorts = map[string]int{
	"next":    3000,
	"fastapi": 8000,
	"go":      8080,
}

func hasComposeMongo(root string) bool {
	composeRaw, ok := readIfPresent(filepath.Join(root, "docker-compose.yml"))
	return ok && strings.Contains(composeRaw, "mongo")
}

func collectManifests(root string) []string {
	var paths []string
	//nolint:errcheck // best-effort collection; partial results acceptable
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".next":
				return fs.SkipDir
			}
			return nil
		}
		switch d.Name() {
		case "package.json", "requirements.txt", "pyproject.toml":
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

func anyManifestHasDriver(paths []string) (string, bool) {
	for _, m := range paths {
		data, readErr := os.ReadFile(m)
		if readErr != nil {
			continue
		}
		lower := strings.ToLower(string(data))
		for _, dep := range []string{"mongoose", "mongodb", "pymongo", "motor"} {
			if strings.Contains(lower, dep) {
				return dep + " dependency in " + m, true
			}
		}
	}
	return "", false
}

func envHasMongoKey(root string) (string, bool) {
	envRaw, ok := readIfPresent(filepath.Join(root, ".env.example"))
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(envRaw, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		upper := strings.ToUpper(fields[0])
		if strings.HasPrefix(upper, "MONGO") &&
			(strings.Contains(upper, "URI") || strings.Contains(upper, "URL") ||
				strings.Contains(upper, "HOST")) {
			return fields[0], true
		}
	}
	return "", false
}

// DetectMongoDB applies the R6 three-way check at repository root.
func DetectMongoDB(root string) MongoEvidence {
	if hasComposeMongo(root) {
		return MongoEvidence{Found: true, Evidence: "docker-compose mongo service"}
	}
	manifests := collectManifests(root)
	if evidence, hit := anyManifestHasDriver(manifests); hit {
		return MongoEvidence{Found: true, Evidence: evidence}
	}
	if key, hit := envHasMongoKey(root); hit {
		return MongoEvidence{Found: true, Evidence: key + " in .env.example"}
	}
	return MongoEvidence{}
}
