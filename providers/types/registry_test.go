package types

import (
	"context"
	"errors"
	"testing"
)

type stubDatabase struct{}

func (stubDatabase) CreateAppDatabase(context.Context, string) (*Credential, error) {
	return &Credential{Username: "app_x"}, nil
}
func (stubDatabase) DeleteAppDatabase(context.Context, string) error { return nil }
func (stubDatabase) Healthz(context.Context) error                   { return nil }

// Compile-time injectability proof: mocks satisfy the contract.
var _ DatabaseProvider = stubDatabase{}

func TestRegistryRoundTrip(t *testing.T) {
	reg := NewRegistry[DatabaseProvider]("database")
	reg.Register("local", func() DatabaseProvider { return stubDatabase{} })

	got, err := reg.New("local")
	if err != nil || got == nil {
		t.Fatalf("local lookup failed: %v", err)
	}
	if _, missErr := reg.New("baidu"); !errors.Is(missErr, ErrUnsupported) {
		t.Fatalf("unknown name must wrap ErrUnsupported, got %v", missErr)
	}
}

func TestPackageRegistriesStartEmpty(t *testing.T) {
	if _, err := Databases.New("local"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("types package must not self-register implementations")
	}
	if _, err := Secrets.New("local"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("secret registry must start empty")
	}
}
