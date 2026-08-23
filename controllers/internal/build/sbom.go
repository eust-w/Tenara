package build

import (
	"errors"
	"fmt"
)

const (
	syftImage      = "anchore/syft:v1.18.1"
	sbomOutputPath = "/out/sbom.json"
	sbomSuffix     = "sbom.json"
)

// SBOMObjectKey returns the MinIO object key storing a build's SBOM document.
func SBOMObjectKey(buildID string) string {
	return fmt.Sprintf("artifacts/%s/%s", buildID, sbomSuffix)
}

// ScanRef pins an image reference to an immutable digest for supply-chain
// steps, preventing TOCTOU between push and scanning.
func ScanRef(tag, digest string) string {
	if tag == "" || digest == "" {
		return ""
	}
	return tag + "@" + digest
}

// SyftScanArgs returns the syft CLI arguments producing an SPDX JSON document.
func SyftScanArgs(scanRef string) []string {
	return []string{"scan", scanRef, "-o", "spdx-json=" + sbomOutputPath}
}

// SetSBOMRef records the artifact reference of the generated SBOM.
func SetSBOMRef(b *Build, ref string) { b.Status.SBOMRef = ref }

// RequireSBOMRef enforces RB-13: a build without an SBOM must never be
// reported as successful downstream.
func RequireSBOMRef(b *Build) error {
	if b.Status.SBOMRef == "" {
		MarkFailed(b, "sbom-missing", "no SBOM artifact recorded; success forbidden")
		return errors.New("sbom-missing")
	}
	return nil
}
