// Package types defines the data-plane provider contracts (RB§40). Core code
// depends on these interfaces only; concrete local/baidu implementations
// register themselves without ever leaking their types into signatures.
package types

import (
	"context"
	"errors"
)

var (
	// ErrUnsupported marks operations or names this provider does not serve.
	ErrUnsupported = errors.New("unsupported provider operation")
	// ErrUnavailable marks a reachable-but-failing backend dependency.
	ErrUnavailable = errors.New("provider backend unavailable")
)

// Credential carries per-app secret material produced by provisioning calls.
// Admin-level credentials never travel through this type.
type Credential struct {
	Username string
	Password string
	URI      string
}

// HealthReporter is embedded by every provider kind.
type HealthReporter interface {
	Healthz(ctx context.Context) error
}

// DatabaseProvider provisions per-app databases with scoped users.
type DatabaseProvider interface {
	CreateAppDatabase(ctx context.Context, appID string) (*Credential, error)
	DeleteAppDatabase(ctx context.Context, appID string) error
	HealthReporter
}

// CacheProvider provisions ACL-scoped cache identities.
type CacheProvider interface {
	CreateAppCache(ctx context.Context, appID string) (*Credential, error)
	DeleteAppCache(ctx context.Context, appID string) error
	HealthReporter
}

// StorageProvider provisions prefix-scoped object storage identities.
type StorageProvider interface {
	CreateAppStorage(ctx context.Context, appID string) (*Credential, error)
	DeleteAppStorage(ctx context.Context, appID string) error
	HealthReporter
}

// SecretProvider seals platform secrets and injects them into tenant namespaces.
type SecretProvider interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, sealed []byte) ([]byte, error)
	InjectToNamespace(ctx context.Context, appID, namespace string, secrets map[string][]byte) error
}

// RegistryProvider resolves immutable digests and verifies signatures; tag
// references never escape to callers (R3).
type RegistryProvider interface {
	ResolveDigest(ctx context.Context, repo, tagOrSHA string) (string, error)
	CheckSignature(ctx context.Context, digest string) (bool, error)
	DeleteImage(ctx context.Context, repo, digest string) error
	HealthReporter
}

// RuntimeProvider binds tenant workloads to runtime guarantees (P2 hooks).
type RuntimeProvider interface {
	BindWorkload(ctx context.Context, appID, namespace string) error
	UnbindWorkload(ctx context.Context, appID, namespace string) error
	HealthReporter
}

// NetworkProvider attaches ingress routing for tenant applications.
type NetworkProvider interface {
	BindDomain(ctx context.Context, appID, host string) error
	UnbindDomain(ctx context.Context, appID, host string) error
	HealthReporter
}

// DNSProvider manages domain-verification challenge records.
type DNSProvider interface {
	CreateChallengeRecord(ctx context.Context, host, value string) error
	DeleteChallengeRecord(ctx context.Context, host, value string) error
	HealthReporter
}
