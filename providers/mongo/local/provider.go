// Package local implements DatabaseProvider against the single-instance
// compose MongoDB using per-app SCRAM users (RB§19). Admin credentials come
// from process env and never leave this package's boundary.
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"

	"tenara/providers/types"
)

const (
	userPrefix    = "app_"
	scramSHA256   = "SCRAM-SHA-256"
	mongoshBinEnv = "TENARA_MONGOSH_BIN"
	defaultBin    = "mongosh"
	adminAuthDB   = "admin"

	appIDPattern = `^[a-z0-9]{3,40}$`
)

var appIDRE = regexp.MustCompile(appIDPattern)

// Config carries admin credentials sourced strictly from process env.
type Config struct {
	Host          string
	AdminUser     string
	AdminPassword string
}

// Runner executes administrative commands against the deployment.
type Runner interface {
	Run(ctx context.Context, cfg *Config, args []string) (string, error)
}

type mongoshRunner struct{}

func (mongoshRunner) Run(ctx context.Context, cfg *Config, args []string) (string, error) {
	bin := os.Getenv(mongoshBinEnv)
	if bin == "" {
		bin = defaultBin
	}
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput() //nolint:gosec // operator override
	return string(out), err
}

// Provider provisions one database plus one readWrite SCRAM user per app.
type Provider struct {
	cfg    *Config
	runner Runner
	random func() (string, error)
}

// New builds the provider from process environment.
func New() types.DatabaseProvider {
	return NewWith(Config{
		Host:          os.Getenv("TENARA_MONGO_HOST"),
		AdminUser:     os.Getenv("TENARA_MONGO_ADMIN_USER"),
		AdminPassword: os.Getenv("TENARA_MONGO_ADMIN_PASSWORD"),
	}, mongoshRunner{}, randomHex32)
}

// NewWith lets tests inject config, runner and randomness.
func NewWith(cfg Config, runner Runner, random func() (string, error)) *Provider {
	return &Provider{cfg: &cfg, runner: runner, random: random}
}

func init() {
	types.Databases.Register("local", func() types.DatabaseProvider { return New() })
}

func randomHex32() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func dbName(appID string) (string, error) {
	if !appIDRE.MatchString(appID) {
		return "", fmt.Errorf("invalid app id %q: want %s", appID, appIDPattern)
	}
	return userPrefix + appID, nil
}

func (p *Provider) CreateAppDatabase(ctx context.Context, appID string) (*types.Credential, error) {
	name, err := dbName(appID)
	if err != nil {
		return nil, err
	}
	pw, err := p.random()
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}
	js := fmt.Sprintf(`db.createUser({user:%q,pwd:%q,roles:[{role:"readWrite",db:%q}],mechanisms:[%q]})`,
		name, pw, name, scramSHA256)
	if _, runErr := p.runner.Run(ctx, p.cfg, p.adminArgs(adminAuthDB, "--eval", js)); runErr != nil {
		return nil, fmt.Errorf("%w: create user %s: %w", types.ErrUnavailable, name, runErr)
	}
	return credential(p.cfg.Host, name, pw), nil
}

func (p *Provider) DeleteAppDatabase(ctx context.Context, appID string) error {
	name, err := dbName(appID)
	if err != nil {
		return err
	}
	js := fmt.Sprintf(`db.getSiblingDB(%q).runCommand({dropUser:%q}); db.getSiblingDB(%q).runCommand({dropDatabase:1})`,
		name, name, name)
	if _, runErr := p.runner.Run(ctx, p.cfg, p.adminArgs(adminAuthDB, "--eval", js)); runErr != nil {
		return fmt.Errorf("%w: drop %s: %w", types.ErrUnavailable, name, runErr)
	}
	return nil
}

func (p *Provider) Healthz(ctx context.Context) error {
	if _, err := p.runner.Run(ctx, p.cfg, p.adminArgs(adminAuthDB, "--eval", "db.runCommand({ping:1})")); err != nil {
		return fmt.Errorf("%w: ping: %w", types.ErrUnavailable, err)
	}
	return nil
}

func (p *Provider) adminArgs(authDB string, extra ...string) []string {
	args := make([]string, 0, 9+len(extra))
	args = append(args,
		"--quiet",
		"--host", p.cfg.Host,
		"-u", p.cfg.AdminUser,
		"-p", p.cfg.AdminPassword,
		"--authenticationDatabase", authDB,
	)
	return append(args, extra...)
}

func credential(host, name, pw string) *types.Credential {
	u := url.URL{
		Scheme: "mongodb",
		User:   url.UserPassword(name, pw),
		Host:   host,
		Path:   "/" + name,
	}
	q := u.Query()
	q.Set("authSource", name)
	u.RawQuery = q.Encode()
	return &types.Credential{Username: name, Password: pw, URI: u.String()}
}
