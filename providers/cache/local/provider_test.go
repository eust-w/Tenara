package local

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tenara/providers/types"
)

const (
	adminPass = "redis-admin-pw"
	rdHost    = "127.0.0.1"
	rdPort    = "6379"
	stubPW    = "abcdef0123456789abcdef0123456789"
)

type fakeRunner struct {
	respErr error
	gotArgs [][]string
}

func (f *fakeRunner) Run(_ context.Context, _ *Config, args []string) (string, error) {
	f.gotArgs = append(f.gotArgs, args)
	return "", f.respErr
}

func newTestProvider(f *fakeRunner) *Provider {
	return NewWith(
		Config{Host: rdHost, Port: rdPort, AdminPassword: adminPass},
		f,
		func() (string, error) { return stubPW, nil },
	)
}

func assertNoAdminLeak(t *testing.T, cred *types.Credential) {
	t.Helper()
	if strings.Contains(cred.URI, adminPass) || strings.Contains(cred.Password, adminPass) {
		t.Fatal("admin password leaked into Credential")
	}
	if !strings.HasPrefix(cred.Username, "cache_") {
		t.Fatalf("shared/default user leaked: %q", cred.Username)
	}
}

func TestCreateAppCacheTokenOrder(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	cred, err := p.CreateAppCache(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"ACL", "SETUSER", "cache_acme",
		"reset",
		"on", ">" + stubPW,
		"~cache_acme:*",
		"+@read", "+@write", "+@connection",
		"-@admin", "-@dangerous",
	}
	got := f.gotArgs[0]
	if len(got) != len(want) {
		t.Fatalf("args = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	if cred.Username != "cache_acme" || cred.Password != stubPW {
		t.Fatalf("credential = %+v", cred)
	}
	wantURI := "redis://cache_acme:" + stubPW + "@" + rdHost + ":" + rdPort
	if cred.URI != wantURI {
		t.Fatalf("uri = %s, want %s", cred.URI, wantURI)
	}
	assertNoAdminLeak(t, cred)
}

func TestDeleteAppCacheDelUser(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	if err := p.DeleteAppCache(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
	got := f.gotArgs[0]
	if len(got) != 3 || got[0] != "ACL" || got[1] != "DELUSER" || got[2] != "cache_acme" {
		t.Fatalf("deluser args = %v", got)
	}
}

func TestRejectsUnsafeAppID(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	for _, bad := range []string{"", "ab", "Acme", "a-b_c!", "has space"} {
		if _, err := p.CreateAppCache(context.Background(), bad); err == nil {
			t.Fatalf("id %q must be rejected", bad)
		}
		if err := p.DeleteAppCache(context.Background(), bad); err == nil {
			t.Fatalf("delete id %q must be rejected", bad)
		}
	}
	if len(f.gotArgs) != 0 {
		t.Fatal("runner must never execute for invalid ids")
	}
}

func TestHealthzWrapsFailureAsUnavailable(t *testing.T) {
	f := &fakeRunner{respErr: errors.New("connection refused")}
	p := newTestProvider(f)

	if err := p.Healthz(context.Background()); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestRandomHex32Shape(t *testing.T) {
	pw, err := randomHex32()
	if err != nil {
		t.Fatal(err)
	}
	if len(pw) != 32 {
		t.Fatalf("password length = %d, want 32", len(pw))
	}
	for _, r := range pw {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Fatalf("non-hex character %q", r)
		}
	}
}
