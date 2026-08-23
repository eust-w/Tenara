package build

import (
	"errors"
	"fmt"
	"strings"
)

const (
	cosignImage    = "gcr.io/projectsigstore/cosign:v2.4.1"
	tlogUploadFlag = "--tlog-upload=false"
	insecureRegFlg = "--allow-insecure-registry"
)

// RequireDigestRef rejects tag-only references: signing and verification must
// always pin the immutable digest (R3).
func RequireDigestRef(ref string) error {
	if ref == "" {
		return errors.New("empty image reference")
	}
	if !strings.Contains(ref, "@sha256:") {
		return fmt.Errorf("reference %q is not digest-pinned", ref)
	}
	return nil
}

// SignArgs returns the cosign arguments signing a digest-pinned image with a
// local key. Transparency-log upload stays disabled for offline environments.
func SignArgs(keyRef, digestRef string) ([]string, error) {
	if err := RequireDigestRef(digestRef); err != nil {
		return nil, err
	}
	return []string{"sign", "--key", keyRef, tlogUploadFlag, digestRef}, nil
}

// VerifyArgs returns the cosign arguments re-verifying a signed digest-pinned
// image against the same key over the plaintext local registry.
func VerifyArgs(keyRef, digestRef string) ([]string, error) {
	if err := RequireDigestRef(digestRef); err != nil {
		return nil, err
	}
	return []string{"verify", "--key", keyRef, insecureRegFlg, digestRef}, nil
}

// SetSignatureRef records where the image signature artifact lives.
func SetSignatureRef(b *Build, ref string) { b.Status.SignatureRef = ref }

// FinalizeBuild transitions the build to PUSHED only once every supply-chain
// artifact (image digest, SBOM, scan report, signature) is present.
func FinalizeBuild(b *Build) error {
	var missing []string
	if b.Status.ImageDigest == "" {
		missing = append(missing, "digest")
	}
	if b.Status.SBOMRef == "" {
		missing = append(missing, "sbom")
	}
	if b.Status.ScanReportRef == "" {
		missing = append(missing, "scan")
	}
	if b.Status.SignatureRef == "" {
		missing = append(missing, "signature")
	}
	if len(missing) > 0 {
		return fmt.Errorf("cannot finalize build %s: missing %s",
			b.Name, strings.Join(missing, ","))
	}
	b.Status.Phase = PhasePushed
	return nil
}
