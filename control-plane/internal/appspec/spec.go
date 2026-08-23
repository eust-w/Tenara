// Package appspec implements AppSpec v1 parsing and strict validation
// (plan tenara-agent-paas#17, RB-10 R8). Images never appear here: they are
// produced by the build plane only (RB-13), so unknown fields including
// image are rejected at decode time.
package appspec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidSpec = errors.New("invalid appspec")

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

type Spec struct {
	Services map[string]ServiceDef  `json:"services"`
	Routing  map[string]RouteTarget `json:"routing,omitempty"`
	Database *DatabaseDef           `json:"database,omitempty"`
	Version  string                 `json:"version"`
}

var validTypes = map[string]bool{"frontend": true, "backend": true}
var validRuntimes = map[string]bool{"node": true, "python": true, "go": true}

// Parse strictly decodes and validates an AppSpec payload.
func Parse(data []byte) (Spec, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Spec{}, fmt.Errorf("%w: empty document", ErrInvalidSpec)
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var s Spec
	if decodeErr := dec.Decode(&s); decodeErr != nil {
		return Spec{}, fmt.Errorf("%w: %w", ErrInvalidSpec, decodeErr)
	}
	return s, s.validate()
}

func (s Spec) validate() error {
	if s.Version != "v1" {
		return fmt.Errorf("%w: unsupported version %q (only v1)", ErrInvalidSpec, s.Version)
	}
	if len(s.Services) == 0 {
		return fmt.Errorf("%w: at least one service required", ErrInvalidSpec)
	}
	for name, svc := range s.Services {
		if !validTypes[svc.Type] {
			return fmt.Errorf("%w: service %q has invalid type %q", ErrInvalidSpec, name, svc.Type)
		}
		if !validRuntimes[svc.Runtime] {
			return fmt.Errorf("%w: service %q has invalid runtime %q", ErrInvalidSpec, name, svc.Runtime)
		}
		if strings.TrimSpace(svc.Path) == "" {
			return fmt.Errorf("%w: service %q missing path", ErrInvalidSpec, name)
		}
	}
	for route, target := range s.Routing {
		if _, ok := s.Services[target.Service]; !ok {
			return fmt.Errorf("%w: routing %q references unknown service %q",
				ErrInvalidSpec, route, target.Service)
		}
	}
	return nil
}
