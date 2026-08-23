// Package bos implements StorageProvider against Baidu BOS (D2),
// mapping RB§21 prefix-IAM semantics to BOS STS temporary credentials.
package bos

import (
	"context"
	"fmt"

	"tenara/providers/types"
)

// Config carries BOS connection settings from env/Secret sources.
type Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
}

// Doer performs one signed API call. Tests inject fixtures here.
type Doer interface {
	Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error)
}

// Provider implements types.StorageProvider against Baidu BOS.
type Provider struct {
	cfg  Config
	doer Doer
}

// New wires a provider around explicit configuration and transport.
func New(cfg Config, doer Doer) *Provider {
	return &Provider{cfg: cfg, doer: doer}
}

func init() {
	types.Storages.Register("baidu-bos", func() types.StorageProvider {
		return New(Config{}, nil)
	})
}

// CreateAppStorage provisions a scoped storage identity for an app.
func (p *Provider) CreateAppStorage(ctx context.Context, appID string) (*types.Credential, error) {
	return nil, fmt.Errorf("%w: baidu bos CreateAppStorage requires D2 wiring", types.ErrUnavailable)
}

// DeleteAppStorage removes the scoped storage identity for an app.
func (p *Provider) DeleteAppStorage(ctx context.Context, appID string) error {
	return fmt.Errorf("%w: baidu bos DeleteAppStorage requires D2 wiring", types.ErrUnavailable)
}

// Healthz probes BOS reachability.
func (p *Provider) Healthz(ctx context.Context) error {
	status, _, err := p.doer.Do(ctx, &p.cfg, "HEAD", "/", nil)
	if err != nil || status >= 400 {
		return fmt.Errorf("bos healthz: status=%d err=%v", status, err)
	}
	return nil
}
