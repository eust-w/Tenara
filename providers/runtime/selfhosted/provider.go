// Package selfhosted adopts customer-operated Kubernetes clusters instead of
// provisioning them: bring-your-own kube face, Envoy Gateway pre-installed.
package selfhosted

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"tenara/providers/types"
)

// Config points at the adopted cluster API surface; empty Endpoint makes
// every call fail closed at request construction.
type Config struct {
	Endpoint string
}

func (c Config) endpoint() string { return c.Endpoint }

// Provider implements types.RuntimeProvider over an existing cluster.
type Provider struct {
	cfg Config
}

// New wires the adapter around one adopted endpoint.
func New(cfg Config) *Provider { return &Provider{cfg: cfg} }

var errProvisionUnsupported = errors.New("self-hosted runtime adopts clusters; provisioning unsupported")

// ClusterSpec exists for interface symmetry only.
type ClusterSpec struct {
	Name      string
	Version   string
	NodeCount int
}

// ping performs a plain GET against the cluster face.
func (p *Provider) ping(ctx context.Context, path string) error {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.endpoint()+path, nil)
	if reqErr != nil {
		return fmt.Errorf("%w: %s: %w", types.ErrUnavailable, path, reqErr)
	}
	client := &http.Client{}
	resp, doErr := client.Do(req)
	if doErr != nil {
		return fmt.Errorf("%w: %s: %w", types.ErrUnavailable, path, doErr)
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil {
			_ = cErr //nolint:errcheck // nothing actionable on close failure
		}
	}()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: readyz returned %d", types.ErrUnavailable, resp.StatusCode)
	}
	return nil
}

// CreateCluster refuses provisioning semantics outright.
func (p *Provider) CreateCluster(context.Context, ClusterSpec) (string, error) {
	return "", errProvisionUnsupported
}

// GetCluster maps a healthy adopted face onto the adopted state label.
func (p *Provider) GetCluster(ctx context.Context, _ string) (string, error) {
	if err := p.ping(ctx, "/readyz"); err != nil {
		return "", err
	}
	return "adopted", nil
}

// CreateNodePool refuses provisioning semantics outright.
func (p *Provider) CreateNodePool(context.Context, string, string, int) error {
	return errProvisionUnsupported
}

// DeleteCluster refuses teardown of customer-owned clusters through Tenara.
func (p *Provider) DeleteCluster(context.Context, string) error {
	return errProvisionUnsupported
}

// InstallGateway assumes Envoy Gateway ships pre-installed on adopted faces.
func (p *Provider) InstallGateway(context.Context, string) error { return nil }

// BindWorkload is a D2 wiring point on adopted faces.
func (p *Provider) BindWorkload(context.Context, string, string) error { return nil }

// UnbindWorkload is a D2 wiring point on adopted faces.
func (p *Provider) UnbindWorkload(context.Context, string, string) error { return nil }

// Healthz probes the adopted cluster readiness endpoint.
func (p *Provider) Healthz(ctx context.Context) error {
	return p.ping(ctx, "/readyz")
}

// RuntimeProvider compliance for registry registration.
var _ types.RuntimeProvider = (*Provider)(nil)

func init() {
	types.Runtimes.Register("selfhosted", func() types.RuntimeProvider {
		return New(Config{})
	})
}
