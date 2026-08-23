package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tenara/analyzer/internal/boundary"
	"github.com/tenara/analyzer/internal/detectors"
	"github.com/tenara/analyzer/internal/generator"
)

// runAppspecStage executes the full detection pipeline and prints AppSpec v1.
func runAppspecStage(root string) error {
	clean := filepath.Clean(root)

	b, bErr := boundary.Detect(clean)
	if bErr != nil {
		return fmt.Errorf("boundary: %w", bErr)
	}

	svcFacts := map[string]detectors.ServiceFacts{}
	for _, svc := range b.Services {
		svcDir := filepath.Join(clean, filepath.FromSlash(svc.Path))
		svcFacts[svc.Path] = detectors.DetectService(svcDir)
	}
	mongo := detectors.DetectMongoDB(clean)

	slug := strings.SplitN(filepath.Base(clean), "-", 2)[0]
	result, genErr := generator.Generate(svcFacts, mongo, slug, "tenara.local")
	if genErr != nil {
		return fmt.Errorf("generate: %w", genErr)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
