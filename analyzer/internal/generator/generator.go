// Package generator renders AppSpec v1 from detection results (RB-10 R8).
package generator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"tenara/analyzer/internal/detectors"
)

var ErrUnsupportedStack = errors.New("unsupported stack")

var supportedRuntimes = []string{"node", "python", "go"}

type ServiceDef struct {
	Type    string `json:"type"`
	Runtime string `json:"runtime"`
	Path    string `json:"path"`
	Port    int    `json:"port,omitempty"`
}

type DatabaseDef struct {
	MongoDB bool `json:"mongodb"`
}

type RouteTarget struct {
	Service string `json:"service"`
}

type AppSpecV1 struct {
	Services map[string]ServiceDef  `json:"services"`
	Routing  map[string]RouteTarget `json:"routing,omitempty"`
	Database *DatabaseDef           `json:"database,omitempty"`
	Version  string                 `json:"version"`
}

type ConfidenceEntry struct {
	Field    string   `json:"field"`
	Decision string   `json:"decision"`
	Evidence []string `json:"evidence"`
}

type Result struct {
	Spec       AppSpecV1         `json:"app_spec"`
	Confidence []ConfidenceEntry `json:"confidence"`
}

func unsupportedError(dirs []string) error {
	return fmt.Errorf("%w: %s; supported runtimes: %s",
		ErrUnsupportedStack, strings.Join(dirs, ", "),
		strings.Join(supportedRuntimes, ", "))
}

// Generate renders AppSpec v1 from per-dir detection results.
func Generate(
	services map[string]detectors.ServiceFacts,
	mongo detectors.MongoEvidence,
	slug, baseDomain string,
) (*Result, error) {
	spec := AppSpecV1{Version: "v1", Services: map[string]ServiceDef{}}
	confidence := make([]ConfidenceEntry, 0, 8)
	var unsupported []string

	for dir, facts := range services {
		switch facts.Runtime {
		case "node", "python", "go":
			svcType := "backend"
			if facts.Framework == "next" || facts.Framework == "vue" ||
				facts.Framework == "react" || facts.Framework == "vite" ||
				facts.Framework == "nuxt" {
				svcType = "frontend"
			}
			spec.Services[dir] = ServiceDef{
				Type: svcType, Runtime: facts.Runtime,
				Path: dir, Port: facts.Port,
			}
		default:
			unsupported = append(unsupported, dir)
		}
	}
	if len(unsupported) > 0 {
		return nil, unsupportedError(unsupported)
	}
	if len(spec.Services) == 0 {
		return nil, fmt.Errorf("%w: no services detected", ErrUnsupportedStack)
	}

	confidence = append(confidence, buildRouting(&spec, services)...)
	confidence = append(confidence, buildDatabase(&spec, mongo)...)
	return &Result{Spec: spec, Confidence: confidence}, nil
}

func buildRouting(spec *AppSpecV1, _ map[string]detectors.ServiceFacts) []ConfidenceEntry {
	routing := map[string]RouteTarget{}
	var entries []ConfidenceEntry
	for name, svc := range spec.Services {
		if svc.Type != "frontend" {
			continue
		}
		routing["/"] = RouteTarget{Service: name}
		entries = append(entries, ConfidenceEntry{
			Field: "routing./", Decision: name,
			Evidence: []string{name + " is type=frontend"},
		})
		break
	}
	for name, svc := range spec.Services {
		if svc.Type != "backend" {
			continue
		}
		prefix := "/api"
		if _, exists := routing[prefix]; !exists {
			routing[prefix] = RouteTarget{Service: name}
			entries = append(entries, ConfidenceEntry{
				Field: "/api", Decision: name,
				Evidence: []string{name + " is type=backend"},
			})
		}
		break
	}
	if len(routing) == len(spec.Services)+1 && len(spec.Services) == 1 {
		for name := range spec.Services {
			routing["/"] = RouteTarget{Service: name}
		}
	}
	spec.Routing = routing
	return entries
}

func buildDatabase(spec *AppSpecV1, mongo detectors.MongoEvidence) []ConfidenceEntry {
	if mongo.Found {
		spec.Database = &DatabaseDef{MongoDB: true}
	}
	return []ConfidenceEntry{{
		Field: "database.mongodb", Decision: strconv.FormatBool(mongo.Found),
		Evidence: []string{mongo.Evidence},
	}}
}
