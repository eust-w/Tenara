package selfhosted

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"tenara/providers/types"
)

func TestAdoptOnlySemantics(t *testing.T) {
	p := New(Config{Endpoint: "https://kube.internal"})
	ctx := context.Background()
	if _, err := p.CreateCluster(ctx,
		ClusterSpec{Name: "x", Version: "v", NodeCount: 1}); err == nil {
		t.Fatal("provisioning must be refused")
	}
	if err := p.CreateNodePool(ctx, "c", "pool", 1); err == nil {
		t.Fatal("node-pool provisioning must be refused")
	}
	if err := p.DeleteCluster(ctx, "c"); err == nil {
		t.Fatal("teardown of customer clusters must be refused")
	}
}

func TestReadyzDrivesStatusAndHealthz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	p := New(Config{Endpoint: srv.URL})
	ctx := context.Background()
	st, err := p.GetCluster(ctx, "ignored")
	if err != nil || st != "adopted" {
		t.Fatalf("get = %q %v", st, err)
	}
	if hErr := p.Healthz(ctx); hErr != nil {
		t.Fatalf("healthz: %v", hErr)
	}
}

func TestUnreachableFailsClosed(t *testing.T) {
	p := New(Config{})
	if err := p.Healthz(context.Background()); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("empty endpoint must wrap sentinel: %v", err)
	}
}
