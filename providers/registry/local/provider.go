// Package local implements RegistryProvider against the compose registry,
// enforcing the digest-only contract (RB§13 R3): tag references never escape
// to callers.
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"tenara/providers/types"
)

const (
	binEnv     = "TENARA_COSIGN_BIN"
	defaultBin = "cosign"

	defaultEndpoint = "http://registry.tenara.local:5000"
	keyEnv          = "TENARA_COSIGN_KEY"

	digestPattern = `^sha256:[a-f0-9]{64}$`

	acceptDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"
	acceptOCIManifest    = "application/vnd.oci.image.manifest.v1+json"
	acceptDockerList     = "application/vnd.docker.distribution.manifest.list.v2+json"
	acceptOCIIndex       = "application/vnd.oci.image.index.v1+json"
)

var (
	digestRE       = regexp.MustCompile(digestPattern)
	manifestAccept = strings.Join([]string{
		acceptDockerManifest, acceptOCIManifest, acceptDockerList, acceptOCIIndex,
	}, ", ")
)

// Config carries the registry endpoint plus the cosign verification key.
type Config struct {
	Endpoint     string
	CosignKeyRef string
}

// Runner executes signature commands against the deployment.
type Runner interface {
	Run(ctx context.Context, cfg *Config, args []string) (string, error)
}

type cosignRunner struct{}

func (cosignRunner) Run(ctx context.Context, cfg *Config, args []string) (string, error) {
	bin := os.Getenv(binEnv)
	if bin == "" {
		bin = defaultBin
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput() //nolint:gosec // operator override
	return string(out), err
}

// Provider resolves immutable digests, verifies signatures and deletes
// manifests over the registry v2 API.
type Provider struct {
	cfg    *Config
	http   *http.Client
	runner Runner
}

// New builds the provider from process environment.
func New() types.RegistryProvider {
	client := &http.Client{}
	return NewWith(Config{
		Endpoint:     envOr("TENARA_REGISTRY_ENDPOINT", defaultEndpoint),
		CosignKeyRef: os.Getenv(keyEnv),
	}, client, cosignRunner{})
}

// NewWith lets tests inject config, HTTP transport and the signing runner.
func NewWith(cfg Config, client *http.Client, runner Runner) *Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{cfg: &cfg, http: client, runner: runner}
}

func init() {
	types.Registries.Register("local", func() types.RegistryProvider { return New() })
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func manifestURL(endpoint, repo, ref string) string {
	return fmt.Sprintf("%s/v2/%s/manifests/%s",
		strings.TrimSuffix(endpoint, "/"), repo, ref)
}

// manifestResponse carries everything our flows consume: bodies hold no
// payload, so they are drained and closed with checked errors up front.
type manifestResponse struct {
	header http.Header
	status int
}

func (p *Provider) do(ctx context.Context, method, target string) (*manifestResponse, error) {
	req, rErr := http.NewRequestWithContext(ctx, method, target, nil)
	if rErr != nil {
		return nil, fmt.Errorf("build request: %w", rErr)
	}
	req.Header.Set("Accept", manifestAccept)
	resp, dErr := p.http.Do(req)
	if dErr != nil {
		return nil, fmt.Errorf("%w: %w", types.ErrUnavailable, dErr)
	}
	_, copyErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	if copyErr != nil || closeErr != nil {
		return nil, fmt.Errorf("%w: body read/close: %w",
			types.ErrUnavailable, errors.Join(copyErr, closeErr))
	}
	return &manifestResponse{status: resp.StatusCode, header: resp.Header}, nil
}

// ResolveDigest normalizes any tag or digest reference into the canonical
// sha256 digest taken from the registry response header.
func (p *Provider) ResolveDigest(ctx context.Context, repo, tagOrSHA string) (string, error) {
	resp, err := p.do(ctx, http.MethodGet, manifestURL(p.cfg.Endpoint, repo, tagOrSHA))
	if err != nil {
		return "", err
	}
	if resp.status != http.StatusOK {
		return "", fmt.Errorf("%w: resolve %s/%s: status %d",
			types.ErrUnavailable, repo, tagOrSHA, resp.status)
	}
	digest := strings.TrimSpace(resp.header.Get("Docker-Content-Digest"))
	if !digestRE.MatchString(digest) {
		// R3: an unverifiable reference must never leak upward as a tag.
		return "", fmt.Errorf("registry returned no canonical digest for %s/%s", repo, tagOrSHA)
	}
	return digest, nil
}

// CheckSignature runs cosign verify over repo@digest; a nonzero cosign exit
// means unsigned rather than operational failure. Non-digest inputs are
// rejected before execution.
func (p *Provider) CheckSignature(ctx context.Context, repo, digest string) (bool, error) {
	if !digestRE.MatchString(digest) {
		return false, fmt.Errorf("reference %q is not a sha256 digest", digest)
	}
	args := []string{
		"verify",
		"--key", p.cfg.CosignKeyRef,
		"--allow-insecure-registry",
		fmt.Sprintf("%s/%s@%s", strings.TrimSuffix(p.cfg.Endpoint, "/"), repo, digest),
	}
	_, runErr := p.runner.Run(ctx, p.cfg, args)
	return runErr == nil, nil
}

// DeleteImage removes one manifest by digest; layers stay until GC.
func (p *Provider) DeleteImage(ctx context.Context, repo, digest string) error {
	if !digestRE.MatchString(digest) {
		return fmt.Errorf("reference %q is not a sha256 digest", digest)
	}
	resp, err := p.do(ctx, http.MethodDelete, manifestURL(p.cfg.Endpoint, repo, digest))
	if err != nil {
		return err
	}
	if resp.status != http.StatusAccepted {
		return fmt.Errorf("%w: delete %s@%s: status %d",
			types.ErrUnavailable, repo, digest, resp.status)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	target := strings.TrimSuffix(p.cfg.Endpoint, "/") + "/v2/"
	resp, err := p.do(ctx, http.MethodGet, target)
	if err != nil {
		return err
	}
	if resp.status != http.StatusOK {
		return fmt.Errorf("%w: healthz: status %d", types.ErrUnavailable, resp.status)
	}
	return nil
}
