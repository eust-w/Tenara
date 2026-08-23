// Package local implements CacheProvider against the compose Redis using
// ACL-scoped per-app users (RB§20). Admin credentials come from process env
// and never leave this package's boundary.
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"

	"tenara/providers/types"
)

const (
	userPrefix   = "cache_"
	keyFmt       = "~cache_%s:*"
	redisBinEnv  = "TENARA_REDIS_CLI_BIN"
	defaultBin   = "redis-cli"
	appIDPattern = `^[a-z0-9]{3,40}$`
)

var appIDRE = regexp.MustCompile(appIDPattern)

// Config carries admin connection settings sourced strictly from env.
type Config struct {
	Host          string
	Port          string
	AdminPassword string
}

// Runner executes commands against the deployment.
type Runner interface {
	Run(ctx context.Context, cfg *Config, args []string) (string, error)
}

type redisCLIRunner struct{}

func (redisCLIRunner) Run(ctx context.Context, cfg *Config, args []string) (string, error) {
	bin := os.Getenv(redisBinEnv)
	if bin == "" {
		bin = defaultBin
	}
	full := make([]string, 0, len(args)+6)
	full = append(full, "-h", cfg.Host, "-p", cfg.Port)
	if cfg.AdminPassword != "" {
		full = append(full, "-a", cfg.AdminPassword, "--no-auth-warning")
	}
	full = append(full, args...)
	out, err := exec.CommandContext(ctx, bin, full...).CombinedOutput() //nolint:gosec // operator override
	return string(out), err
}

// Provider provisions one ACL user per app restricted to its own key prefix.
type Provider struct {
	cfg    *Config
	runner Runner
	random func() (string, error)
}

// New builds the provider from process environment.
func New() types.CacheProvider {
	return NewWith(Config{
		Host:          os.Getenv("TENARA_REDIS_HOST"),
		Port:          os.Getenv("TENARA_REDIS_PORT"),
		AdminPassword: os.Getenv("TENARA_REDIS_ADMIN_PASSWORD"),
	}, redisCLIRunner{}, randomHex32)
}

// NewWith lets tests inject config, runner and randomness.
func NewWith(cfg Config, runner Runner, random func() (string, error)) *Provider {
	return &Provider{cfg: &cfg, runner: runner, random: random}
}

func init() {
	types.Caches.Register("local", func() types.CacheProvider { return New() })
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

// aclTokens returns the §53.1 gotcha-compliant token sequence: reset comes
// first so re-provisioning can never accumulate permissions incrementally.
func aclTokens(name, id, pw string) []string {
	return []string{
		"ACL", "SETUSER", name,
		"reset",
		"on", ">" + pw,
		fmt.Sprintf(keyFmt, id),
		"+@read", "+@write", "+@connection",
		"-@admin", "-@dangerous",
	}
}

func (p *Provider) CreateAppCache(ctx context.Context, appID string) (*types.Credential, error) {
	name, err := userName(appID)
	if err != nil {
		return nil, err
	}
	pw, err := p.random()
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}
	if _, runErr := p.runner.Run(ctx, p.cfg, aclTokens(name, appID, pw)); runErr != nil {
		return nil, fmt.Errorf("%w: create acl user %s: %w", types.ErrUnavailable, name, runErr)
	}
	u := url.URL{
		Scheme: "redis",
		User:   url.UserPassword(name, pw),
		Host:   net.JoinHostPort(p.cfg.Host, p.cfg.Port),
	}
	return &types.Credential{Username: name, Password: pw, URI: u.String()}, nil
}

func (p *Provider) DeleteAppCache(ctx context.Context, appID string) error {
	name, err := userName(appID)
	if err != nil {
		return err
	}
	if _, runErr := p.runner.Run(ctx, p.cfg, []string{"ACL", "DELUSER", name}); runErr != nil {
		return fmt.Errorf("%w: del acl user %s: %w", types.ErrUnavailable, name, runErr)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	if _, err := p.runner.Run(ctx, p.cfg, []string{"PING"}); err != nil {
		return fmt.Errorf("%w: ping: %w", types.ErrUnavailable, err)
	}
	return nil
}
