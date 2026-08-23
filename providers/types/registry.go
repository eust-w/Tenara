package types

import (
	"fmt"
	"sync"
)

// Registry maps data-plane names ("local", "baidu") to typed provider
// constructors. Concrete packages self-register via init(); core consumes
// instances through these interfaces exclusively (RB§40).
type Registry[T any] struct {
	factories map[string]func() T
	kind      string
	mu        sync.RWMutex
}

// NewRegistry creates a named registry for one provider kind.
func NewRegistry[T any](kind string) *Registry[T] {
	return &Registry[T]{kind: kind, factories: map[string]func() T{}}
}

// Register records a constructor under a data-plane name.
func (r *Registry[T]) Register(name string, ctor func() T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = ctor
}

// New builds an instance; unknown names yield ErrUnsupported.
func (r *Registry[T]) New(name string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var zero T
	ctor, ok := r.factories[name]
	if !ok {
		return zero, fmt.Errorf("%w: no %s provider named %q", ErrUnsupported, r.kind, name)
	}
	return ctor(), nil
}

// Package-level registration points populated solely by implementation
// packages via init(); they intentionally start empty here.
var (
	Runtimes   = NewRegistry[RuntimeProvider]("runtime")
	Registries = NewRegistry[RegistryProvider]("registry")
	Databases  = NewRegistry[DatabaseProvider]("database")
	Caches     = NewRegistry[CacheProvider]("cache")
	Storages   = NewRegistry[StorageProvider]("storage")
	Networks   = NewRegistry[NetworkProvider]("network")
	DNS        = NewRegistry[DNSProvider]("dns")
	Secrets    = NewRegistry[SecretProvider]("secret")
)
