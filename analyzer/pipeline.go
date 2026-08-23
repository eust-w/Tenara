// Package analyzer exposes the full detection pipeline as a public API for
// cross-module consumption by the control plane (plan tenara-agent-paas#27).
package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tenara/analyzer/internal/boundary"
	"github.com/tenara/analyzer/internal/detectors"
	"github.com/tenara/analyzer/internal/facts"
	"github.com/tenara/analyzer/internal/generator"
)

// AnalyzeLocal runs the full detection pipeline on a local repository path
// and returns the AppSpec + confidence report as pre-marshaled JSON.
func AnalyzeLocal(repoPath string, baseDomain string) (json.RawMessage, error) {
	clean := filepath.Clean(repoPath)

	if _, stageErr := facts.Build(clean); stageErr != nil {
		return nil, fmt.Errorf("facts stage: %w", stageErr)
	}

	b, bErr := boundary.Detect(clean)
	if bErr != nil {
		return nil, fmt.Errorf("boundary stage: %w", bErr)
	}

	svcFacts := map[string]detectors.ServiceFacts{}
	for _, svc := range b.Services {
		svcDir := filepath.Join(clean, filepath.FromSlash(svc.Path))
		svcFacts[svc.Path] = detectors.DetectService(svcDir)
	}

	mongo := detectors.DetectMongoDB(clean)

	slug := strings.SplitN(filepath.Base(clean), "-", 2)[0]
	result, genErr := generator.Generate(svcFacts, mongo, slug, baseDomain)
	if genErr != nil {
		return nil, genErr
	}

	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal result: %w", marshalErr)
	}
	return json.RawMessage(raw), nil
}
