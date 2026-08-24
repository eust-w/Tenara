// Package baidu implements DNSProvider against Baidu DNS (D2).
package baidu

import (
	"context"
	"fmt"

	"tenara/providers/types"
)

const isolationShared = "shared"

// Config carries Baidu DNS connection settings.
type Config struct {
	Endpoint string
	ZoneID   string
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
	types.DNS.Register("baidu-dns", func() types.DNSProvider { return New(Config{}, nil) })
}

func (p *Provider) CreateChallengeRecord(ctx context.Context, host, value string) error {
	status, body, err := p.doer.Do(ctx, &p.cfg, "POST",
		fmt.Sprintf("/zone/%s/record?host=%s&type=TXT&rdata=%s", p.cfg.ZoneID, host, value),
		[]byte(fmt.Sprintf(`{"rdata":"%s"}`, value)))
	if err != nil || status >= 400 {
		return fmt.Errorf("create TXT %s: status=%d err=%w", host, status, err)
	}
	_ = body
	return nil
}

func (p *Provider) DeleteChallengeRecord(ctx context.Context, host, value string) error {
	status, _, dErr := p.doer.Do(ctx, &p.cfg, "DELETE",
		fmt.Sprintf("/zone/%s/record?host=%s&type=TXT&rdata=%s", p.cfg.ZoneID, host, value), nil)
	if dErr != nil {
		return fmt.Errorf("delete TXT %s: %w", host, dErr)
	}
	if status >= 400 {
		return fmt.Errorf("delete TXT %s returned %d", host, status)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	status, body, err := p.doer.Do(ctx, &p.cfg, "GET", "/healthz", nil)
	if err != nil || status >= 400 {
		return fmt.Errorf("%w: dns healthz status=%d err=%w", types.ErrUnavailable, status, err)
	}
	_ = body
	return nil
}

var _ = isolationShared
