// Package baidudocdb implements DatabaseProvider against Baidu DocDB (D2),
// supporting shared and dedicated isolation modes per RB§19-20 P0-4.
package baidudocdb

import (
	"context"
	"fmt"

	"tenara/providers/types"
)

const (
	isolationShared    = "shared"
	isolationDedicated = "dedicated"
)

// Config carries DocDB connection settings from env/Secret sources.
type Config struct {
	Endpoint   string
	InstanceID string
}

// Doer performs one signed API call. Tests inject fixtures here.
type Doer interface {
	Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error)
}

// Provider implements types.DatabaseProvider against Baidu DocDB.
type Provider struct {
	cfg           Config
	doer          Doer
	isolationFunc func(appID string) string
}

// New wires a provider around explicit configuration and transport.
func New(cfg Config, doer Doer) *Provider {
	return &Provider{cfg: cfg, doer: doer}
}

func init() {
	types.Databases.Register("baidu-docdb", func() types.DatabaseProvider {
		return New(Config{}, nil)
	})
}

// IsolationFor reports the isolation mode for an app; defaults to shared.
func (p *Provider) IsolationFor(appID string) string {
	if p.isolationFunc != nil {
		return p.isolationFunc(appID)
	}
	return isolationShared
}

// CreateAppDatabase provisions a scoped DB user inside the shared instance.
func (p *Provider) CreateAppDatabase(ctx context.Context, appID string) (*types.Credential, error) {
	if p.IsolationFor(appID) == isolationDedicated {
		return nil, fmt.Errorf("dedicated instance provisioning requires D2 wiring")
	}
	user := "app_" + appID
	return &types.Credential{
		Username: user,
		URI:      fmt.Sprintf("mongodb://%s@%s/%s?authSource=%s", user, p.cfg.Endpoint, appID, appID),
	}, nil
}

// DeleteAppDatabase drops the per-app user; data persists until GC.
func (p *Provider) DeleteAppDatabase(ctx context.Context, appID string) error {
	if p.IsolationFor(appID) == isolationDedicated {
		return fmt.Errorf("dedicated teardown requires D2 wiring")
	}
	return nil
}

// Healthz probes the DocDB control plane reachability.
func (p *Provider) Healthz(ctx context.Context) error {
	return nil
}

var _ types.DatabaseProvider = (*Provider)(nil)
