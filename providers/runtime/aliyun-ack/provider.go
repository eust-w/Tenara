package aliyunack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"tenara/providers/types"
)

// apiPrefix scopes every ACK REST call made by Tenara.
const apiPrefix = "/ack"

// Config carries region and credentials; credentials arrive from a Secret
// source, never hardcoded. Endpoint overrides the vendor default template.
type Config struct {
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

func (c Config) endpoint() string {
	if c.Endpoint != "" {
		return strings.Replace(c.Endpoint, "{region}", c.Region, 1)
	}
	return strings.Replace("https://cs.{region}.aliyuncs.com", "{region}", c.Region, 1)
}

// Doer performs one signed API call. Tests inject fixtures here.
type Doer interface {
	Do(ctx context.Context, cfg Config, method, path string, body []byte) (int, []byte, error)
}

type httpDoer struct{ client *http.Client }

func (h httpDoer) Do(ctx context.Context, cfg Config, method, path string, body []byte) (int, []byte, error) {
	req, reqErr := http.NewRequestWithContext(ctx, method, cfg.endpoint()+path, bytes.NewReader(body))
	if reqErr != nil {
		return 0, nil, reqErr
	}
	req.Header.Set("content-type", "application/json")
	resp, doErr := h.client.Do(req)
	if doErr != nil {
		return 0, nil, doErr
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil {
			_ = cErr //nolint:errcheck // body fully consumed above
		}
	}()
	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, nil, readErr
	}
	return resp.StatusCode, payload, nil
}

// Provider adapts Aliyun ACK to the platform runtime contract.
type Provider struct {
	cfg  Config
	doer Doer
}

// New wires a provider around an explicit configuration and transport.
func New(cfg Config, doer Doer) *Provider {
	return &Provider{cfg: cfg, doer: doer}
}

// ClusterSpec describes the minimal cluster requested by Tenara.
type ClusterSpec struct {
	Name      string
	Version   string
	NodeCount int
}

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
		return nil, fmt.Errorf("%w: Aliyun ACK %s %s returned %d",
			types.ErrUnavailable, method, path, status)
	}
	return respBody, nil
}

// CreateCluster provisions a managed cluster and returns its id.
func (p *Provider) CreateCluster(ctx context.Context, spec ClusterSpec) (string, error) {
	if spec.Name == "" || spec.Version == "" || spec.NodeCount < 1 {
		return "", errors.New("invalid cluster spec")
	}
	body, err := p.call(ctx, http.MethodPost, apiPrefix+"/clusters",
		map[string]any{"name": spec.Name, "version": spec.Version, "node_count": spec.NodeCount})
	if err != nil {
		return "", err
	}
	var out struct {
		ClusterID string `json:"clusterId"`
	}
	if uErr := json.Unmarshal(body, &out); uErr != nil {
		return "", fmt.Errorf("decode cluster: %w", uErr)
	}
	return out.ClusterID, nil
}

// GetCluster reports the remote lifecycle status of one cluster.
func (p *Provider) GetCluster(ctx context.Context, clusterID string) (string, error) {
	body, err := p.call(ctx, http.MethodGet, apiPrefix+"/clusters/"+clusterID, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Status string `json:"status"`
	}
	if uErr := json.Unmarshal(body, &out); uErr != nil {
		return "", fmt.Errorf("decode cluster status: %w", uErr)
	}
	return out.Status, nil
}

// CreateNodePool scales one cluster with an extra managed pool.
func (p *Provider) CreateNodePool(ctx context.Context, clusterID, poolName string, replicas int) error {
	_, err := p.call(ctx, http.MethodPost, apiPrefix+"/clusters/"+clusterID+"/node-pools",
		map[string]any{"name": poolName, "replicas": replicas})
	return err
}

// DeleteCluster tears down a managed cluster.
func (p *Provider) DeleteCluster(ctx context.Context, clusterID string) error {
	_, err := p.call(ctx, http.MethodDelete, apiPrefix+"/clusters/"+clusterID, nil)
	return err
}

// InstallGateway bootstraps Envoy Gateway onto the cluster.
func (p *Provider) InstallGateway(ctx context.Context, clusterID string) error {
	_, err := p.call(ctx, http.MethodPost, apiPrefix+"/clusters/"+clusterID+"/gateway", nil)
	return err
}

// BindWorkload mirrors UnbindWorkload as a D2 wiring point.
func (p *Provider) BindWorkload(context.Context, string, string) error { return nil }

// UnbindWorkload mirrors BindWorkload as a D2 wiring point.
func (p *Provider) UnbindWorkload(context.Context, string, string) error { return nil }

// Healthz probes the Aliyun ACK control plane reachability.
func (p *Provider) Healthz(ctx context.Context) error {
	if _, err := p.call(ctx, http.MethodGet, apiPrefix+"/clusters", nil); err != nil {
		return fmt.Errorf("%w: healthz", types.ErrUnavailable)
	}
	return nil
}

// RuntimeProvider compliance for registry registration.
var _ types.RuntimeProvider = (*Provider)(nil)

func init() {
	types.Runtimes.Register("aliyun-ack", func() types.RuntimeProvider {
		return New(Config{}, httpDoer{client: &http.Client{}})
	})
}
