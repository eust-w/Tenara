// Package baiduccr implements RegistryProvider against Baidu CCR (D2).
// Every request flows through an injected Doer so tests run on fixtures and
// never touch the wire; signing and live requests land with D2 wiring.
package baiduccr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tenara/providers/types"
)

// Config carries the CCR endpoint and credentials from a Secret source.
type Config struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	InstanceID string
}

// Doer performs one signed API call. Tests inject fixtures here.
type Doer interface {
	Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error)
}

// Provider implements RegistryProvider against Baidu CCR.
type Provider struct {
	cfg  *Config
	doer Doer
}

// New wires a provider around an explicit configuration and transport.
func New(cfg *Config, doer Doer) *Provider {
	return &Provider{cfg: cfg, doer: doer}
}

func init() {
	types.Registries.Register("baidu-ccr", func() types.RegistryProvider {
		return New(&Config{}, nil)
	})
}

//nolint:unparam // body kept for Doer symmetry
func (p *Provider) call(ctx context.Context, method, path string, body any) ([]byte, error) {
	var payload []byte
	if body != nil {
		marshaled, mErr := json.Marshal(body)
		if mErr != nil {
			return nil, fmt.Errorf("marshal request: %w", mErr)
		}
		payload = marshaled
	}
	status, respBody, err := p.doer.Do(ctx, p.cfg, method, path, payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", types.ErrUnavailable, method, path, err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("ccr %s %s returned %d", method, path, status)
	}
	return respBody, nil
}

// ResolveDigest normalizes a tag or digest into the canonical sha256 digest.
func (p *Provider) ResolveDigest(ctx context.Context, repo, tagOrSHA string) (string, error) {
	body, err := p.call(ctx, http.MethodGet,
		fmt.Sprintf("/repositories/%s/tags/%s", repo, tagOrSHA), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Digest string `json:"digest"`
	}
	if json.Unmarshal(body, &out) != nil || out.Digest == "" ||
		!strings.HasPrefix(out.Digest, "sha256:") {
		return "", fmt.Errorf("no canonical digest for %s/%s", repo, tagOrSHA)
	}
	return out.Digest, nil
}

// CheckSignature reports whether a digest has been verified by cosign.
func (p *Provider) CheckSignature(ctx context.Context, repo, digest string) (bool, error) {
	if !strings.HasPrefix(digest, "sha256:") {
		return false, fmt.Errorf("reference %q is not a sha256 digest", digest)
	}
	body, err := p.call(ctx, http.MethodGet,
		fmt.Sprintf("/repositories/%s/signatures", repo), nil)
	if err != nil {
		return false, err
	}
	var sigs struct {
		Signatures []struct {
			Digest string `json:"digest"`
		} `json:"signatures"`
	}
	if uErr := json.Unmarshal(body, &sigs); uErr != nil {
		return false, fmt.Errorf("decode signatures: %w", uErr)
	}
	for _, s := range sigs.Signatures {
		if s.Digest == digest {
			return true, nil
		}
	}
	return false, nil
}

// DeleteImage removes one manifest by digest.
func (p *Provider) DeleteImage(ctx context.Context, repo, digest string) error {
	_, err := p.call(ctx, http.MethodDelete,
		fmt.Sprintf("/repositories/%s/manifests/%s", repo, digest), nil)
	return err
}

// Healthz probes the CCR control plane.
func (p *Provider) Healthz(ctx context.Context) error {
	_, err := p.call(ctx, http.MethodGet, "/healthz", nil)
	if err != nil {
		return fmt.Errorf("%w: healthz", types.ErrUnavailable)
	}
	return nil
}

var _ types.RegistryProvider = (*Provider)(nil)
