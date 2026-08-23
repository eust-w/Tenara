// Package baidu implements NetworkProvider against Baidu BLB (D2).
package baidu

import (
	"context"
	"fmt"

	"tenara/providers/types"
)

// Config carries BLB connection settings from env/Secret sources.
type Config struct {
	Endpoint string
	BLBID    string
}

type Provider struct {
	cfg  Config
	doer Doer
}

type Doer interface {
	Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error)
}

func New(cfg Config, doer Doer) *Provider {
	return &Provider{cfg: cfg, doer: doer}
}

func init() {
	types.Networks.Register("baidu", func() types.NetworkProvider { return New(Config{}, nil) })
}

func (p *Provider) BindDomain(ctx context.Context, appID, host string) error {
	status, _, err := p.doer.Do(ctx, &p.cfg, "POST", fmt.Sprintf("/listener?app=%s&host=%s", appID, host), nil)
	if err != nil || status >= 400 {
		return fmt.Errorf("bind %s: status=%d err=%v", host, status, err)
	}
	return nil
}

func (p *Provider) UnbindDomain(ctx context.Context, appID, host string) error {
	status, _, err := p.doer.Do(ctx, &p.cfg, "DELETE", fmt.Sprintf("/listener?app=%s&host=%s", appID, host), nil)
	if err != nil || status >= 400 {
		return fmt.Errorf("unbind %s: status=%d err=%v", host, status, err)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	status, _, err := p.doer.Do(ctx, &p.cfg, "GET", "/healthz", nil)
	if err != nil || status >= 400 {
		return fmt.Errorf("%w: healthz status=%d", types.ErrUnavailable, status)
	}
	return nil
}
