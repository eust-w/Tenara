package baidu

import (
	"context"
	"testing"
)

func TestBindUnbindRoundTrip(t *testing.T) {
	doerCalls := 0
	doer := fakeNet{onCall: func() { doerCalls++ }}
	p := New(Config{}, doer)
	if err := p.BindDomain(context.Background(), "acme", "app.test"); err != nil {
		t.Fatal(err)
	}
	if err := p.UnbindDomain(context.Background(), "acme", "app.test"); err != nil {
		t.Fatal(err)
	}
	if doerCalls != 2 {
		t.Fatalf("calls = %d, want 2", doerCalls)
	}
}

func TestHealthz(t *testing.T) {
	doer := fakeNet{onCall: func() {}}
	p := New(Config{}, doer)
	if err := p.Healthz(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type fakeNet struct{ onCall func() }

func (f fakeNet) Do(ctx context.Context, cfg *Config, method, path string, body []byte) (int, []byte, error) {
	f.onCall()
	return 200, nil, nil
}

var _ Doer = fakeNet{}
