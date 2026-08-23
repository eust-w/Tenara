package bos

import (
	"context"
	"strings"
	"testing"
)

type fakeDoer struct{}

func (fakeDoer) Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error) {
	return 200, []byte(`{}`), nil
}

func TestNewRegistersCorrectly(t *testing.T) {
	p := New(Config{Endpoint: "https://bos.test"}, fakeDoer{})
	if p == nil {
		t.Fatal("provider must not be nil")
	}
}

func TestHealthzDelegatesToDoer(t *testing.T) {
	p := New(Config{}, fakeDoer{})
	if err := p.Healthz(context.Background()); err != nil {
		t.Fatalf("healthz = %v", err)
	}
}

func TestCreateAndDeleteAreD2Stubs(t *testing.T) {
	p := New(Config{}, fakeDoer{})
	if _, err := p.CreateAppStorage(context.Background(), "a1"); err == nil || !strings.Contains(err.Error(), "D2") {
		t.Fatalf("create must indicate D2 wiring needed, got %v", err)
	}
	if err := p.DeleteAppStorage(context.Background(), "a1"); err == nil || !strings.Contains(err.Error(), "D2") {
		t.Fatalf("delete must indicate D2 wiring needed, got %v", err)
	}
}
