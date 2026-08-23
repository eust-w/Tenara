// Package local implements StorageProvider against compose MinIO using
// prefix-scoped IAM users (RB§21). Admin operations ride a preconfigured mc
// alias whose credentials never leave this package's boundary.
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"

	"tenara/providers/types"
)

const (
	bucketName   = "tenara-tenant"
	userPrefix   = "app_"
	mcBinEnv     = "TENARA_MC_BIN"
	defaultBin   = "mc"
	appIDPattern = `^[a-z0-9]{3,40}$`
)

var appIDRE = regexp.MustCompile(appIDPattern)

// Config selects the preconfigured mc alias to administer.
type Config struct {
	Alias string
}

// Runner executes commands against the deployment.
type Runner interface {
	Run(ctx context.Context, cfg *Config, args []string) (string, error)
}

type mcRunner struct{}

func (mcRunner) Run(ctx context.Context, cfg *Config, args []string) (string, error) {
	bin := os.Getenv(mcBinEnv)
	if bin == "" {
		bin = defaultBin
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput() //nolint:gosec // operator override
	return string(out), err
}

// Provider provisions one IAM user per app scoped to its object prefix.
type Provider struct {
	cfg    *Config
	runner Runner
	random func() (string, error)
}

// New builds the provider from process environment.
func New() types.StorageProvider {
	alias := os.Getenv("TENARA_MC_ALIAS")
	if alias == "" {
		alias = "local"
	}
	return NewWith(Config{Alias: alias}, mcRunner{}, randomHex32)
}

// NewWith lets tests inject config, runner and randomness.
func NewWith(cfg Config, runner Runner, random func() (string, error)) *Provider {
	return &Provider{cfg: &cfg, runner: runner, random: random}
}

func init() {
	types.Storages.Register("local", func() types.StorageProvider { return New() })
}

func randomHex32() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func userName(appID string) (string, error) {
	if !appIDRE.MatchString(appID) {
		return "", fmt.Errorf("invalid app id %q: want %s", appID, appIDPattern)
	}
	return userPrefix + appID, nil
}

// buildPolicyJSON renders the §53.1 template scoping the user to its own
// object prefix; a bucket-level wildcard is structurally impossible here.
func buildPolicyJSON(id string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": ["arn:aws:s3:::%s/%s/*"]
    },
    {
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": ["arn:aws:s3:::%s"],
      "Condition": {"StringLike": {"s3:prefix": "%s/*"}}
    }
  ]
}`, bucketName, userPrefix+id, bucketName, userPrefix+id)
}

func removeIfExists(path string) error {
	if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		return rmErr
	}
	return nil
}

func writeTempPolicy(body string) (string, func() error, error) {
	f, err := os.CreateTemp("", "tenara-policy-*.json")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	_, wErr := f.WriteString(body)
	cErr := f.Close()
	if wErr != nil || cErr != nil {
		return "", nil, errors.Join(wErr, cErr, removeIfExists(path))
	}
	return path, func() error { return removeIfExists(path) }, nil
}

func (p *Provider) CreateAppStorage(ctx context.Context, appID string) (*types.Credential, error) {
	name, err := userName(appID)
	if err != nil {
		return nil, err
	}
	pw, err := p.random()
	if err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}

	policyFile, cleanup, pErr := writeTempPolicy(buildPolicyJSON(appID))
	if pErr != nil {
		return nil, fmt.Errorf("stage policy: %w", pErr)
	}

	provErr := p.provisionSteps(ctx, name, pw, policyFile)
	cleanupErr := cleanup()
	switch {
	case provErr != nil:
		return nil, fmt.Errorf("%w: provision storage %s: %w", types.ErrUnavailable, name, provErr)
	case cleanupErr != nil:
		return nil, fmt.Errorf("cleanup staged policy: %w", cleanupErr)
	}
	return &types.Credential{
		Username: name,
		Password: pw,
		URI:      "s3://" + bucketName + "/" + name,
	}, nil
}

func (p *Provider) provisionSteps(ctx context.Context, name, pw, policyFile string) error {
	for _, args := range [][]string{
		{"admin", "user", "add", p.cfg.Alias, name, pw},
		{"admin", "policy", "add", p.cfg.Alias, name, policyFile},
		{"admin", "policy", "attach", p.cfg.Alias, name, "--user", name},
	} {
		if _, runErr := p.runner.Run(ctx, p.cfg, args); runErr != nil {
			return fmt.Errorf("%w: %w", types.ErrUnavailable, runErr)
		}
	}
	return nil
}

func (p *Provider) DeleteAppStorage(ctx context.Context, appID string) error {
	name, err := userName(appID)
	if err != nil {
		return err
	}
	for _, args := range [][]string{
		{"admin", "policy", "detach", p.cfg.Alias, name, "--user", name},
		{"admin", "user", "remove", p.cfg.Alias, name},
	} {
		if _, runErr := p.runner.Run(ctx, p.cfg, args); runErr != nil {
			return fmt.Errorf("%w: remove storage identity %s: %w", types.ErrUnavailable, name, runErr)
		}
	}
	return nil
}

// EnsureBucket creates the shared tenant bucket when absent.
func (p *Provider) EnsureBucket(ctx context.Context) error {
	if _, err := p.runner.Run(ctx, p.cfg, []string{"mb", p.cfg.Alias + "/" + bucketName}); err != nil {
		return fmt.Errorf("%w: ensure bucket: %w", types.ErrUnavailable, err)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	if _, err := p.runner.Run(ctx, p.cfg, []string{"ls", p.cfg.Alias}); err != nil {
		return fmt.Errorf("%w: ls alias: %w", types.ErrUnavailable, err)
	}
	return nil
}
