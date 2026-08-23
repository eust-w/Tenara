package build

import (
	"strings"
	"testing"
)

func digestPinnedRef() string {
	return ScanRef(ImageTag("app-1", "abcdef1234567890"), "sha256:abc")
}

func TestRequireDigestRef(t *testing.T) {
	if err := RequireDigestRef(digestPinnedRef()); err != nil {
		t.Fatalf("digest ref must pass: %v", err)
	}
	tagOnly := ImageTag("app-1", "abcdef1234567890")
	if err := RequireDigestRef(tagOnly); err == nil {
		t.Fatal("tag-only ref must be rejected")
	}
	if err := RequireDigestRef(""); err == nil {
		t.Fatal("empty ref must be rejected")
	}
}

func TestSignArgs(t *testing.T) {
	ref := digestPinnedRef()
	args, err := SignArgs("kms://stub/build-signing", ref)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")

	for _, want := range []string{"sign", "--key", "kms://stub/build-signing", tlogUploadFlag, ref} {
		if !strings.Contains(joined, want) {
			t.Fatalf("sign args missing %q", want)
		}
	}
	if args[len(args)-1] != ref {
		t.Fatalf("digest ref must be the final arg, got %q", args[len(args)-1])
	}

	if _, tagErr := SignArgs("kms://stub/build-signing", ImageTag("app-1", "abcdef1234567890")); tagErr == nil {
		t.Fatal("tag signing must be rejected")
	}
}

func TestVerifyArgs(t *testing.T) {
	ref := digestPinnedRef()
	args, err := VerifyArgs("kms://stub/build-signing", ref)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")

	for _, want := range []string{"verify", "--key", insecureRegFlg, ref} {
		if !strings.Contains(joined, want) {
			t.Fatalf("verify args missing %q", want)
		}
	}
	if strings.Contains(joined, "--tlog-upload=true") {
		t.Fatal("offline verification must not require tlog upload")
	}

	if _, tagErr := VerifyArgs("kms://stub/build-signing", ImageTag("a", "abcdef1234567890")); tagErr == nil {
		t.Fatal("tag verification must be rejected")
	}
}

func TestFinalizeBuildPushed(t *testing.T) {
	b := sampleBuild()
	b.Status.Phase = PhaseSigning
	b.Status.ImageDigest = "sha256:def456"
	b.Status.SBOMRef = SBOMObjectKey("b1")
	b.Status.ScanReportRef = ScanReportObjectKey("b1")
	SetSignatureRef(b, "artifacts/b1/signature.sig")

	if err := FinalizeBuild(b); err != nil {
		t.Fatalf("complete artifact set must finalize: %v", err)
	}
	if b.Status.Phase != PhasePushed {
		t.Fatalf("phase = %s, want PUSHED", b.Status.Phase)
	}

	partial := sampleBuild()
	partial.Status.Phase = PhaseSigning
	partial.Status.ImageDigest = "sha256:def456"
	finErr := FinalizeBuild(partial)
	if finErr == nil {
		t.Fatal("incomplete artifact set must not finalize")
	}
	for _, want := range []string{"sbom", "scan", "signature"} {
		if !strings.Contains(finErr.Error(), want) {
			t.Fatalf("error must list missing %q: %v", want, finErr)
		}
	}
	if partial.Status.Phase == PhasePushed {
		t.Fatal("phase must remain non-PUSHED on incomplete artifacts")
	}
}
