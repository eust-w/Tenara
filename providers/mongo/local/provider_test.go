package local

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tenara/providers/types"
)

const (
	adminUser = "root"
	adminPass = "sup3rsecret"
	mgoHost   = "127.0.0.1:27017"
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
		Config{Host: mgoHost, AdminUser: adminUser, AdminPassword: adminPass},
		f,
		func() (string, error) { return stubPW, nil },
	)
}

func evalJS(t *testing.T, f *fakeRunner, call int) string {
	t.Helper()
	for i, a := range f.gotArgs[call] {
		if a == "--eval" && i+1 < len(f.gotArgs[call]) {
			return f.gotArgs[call][i+1]
		}
	}
	t.Fatal("no --eval argument captured")
	return ""
}

func assertNoAdminLeak(t *testing.T, cred *types.Credential) {
	t.Helper()
	if strings.Contains(cred.URI, adminPass) || strings.Contains(cred.Password, adminPass) {
		t.Fatal("admin password leaked into Credential")
	}
	if cred.Username == adminUser || strings.Contains(cred.Username, adminUser) {
		t.Fatal("admin username leaked into Credential")
	}
}

func TestCreateAppDatabaseHappyPath(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	cred, err := p.CreateAppDatabase(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}

	js := evalJS(t, f, 0)
	for _, want := range []string{
		`user:"app_acme"`, `"` + stubPW + `"`,
		`role:"readWrite"`, `db:"app_acme"`, `"SCRAM-SHA-256"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("createUser JS missing %s:\n%s", want, js)
		}
	}

	if cred.Username != "app_acme" || cred.Password != stubPW {
		t.Fatalf("credential = %+v", cred)
	}
	wantURI := "mongodb://app_acme:" + stubPW + "@" + mgoHost + "/app_acme?authSource=app_acme"
	if cred.URI != wantURI {
		t.Fatalf("uri = %s, want %s", cred.URI, wantURI)
	}
	assertNoAdminLeak(t, cred)
}

func TestDeleteAppDatabaseDropsUserAndDB(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	if err := p.DeleteAppDatabase(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
	js := evalJS(t, f, 0)
	for _, want := range []string{`getSiblingDB("app_acme")`, `dropUser:"app_acme"`, `{dropDatabase:1}`} {
		if !strings.Contains(js, want) {
			t.Fatalf("delete JS missing %s:\n%s", want, js)
		}
	}
}

func TestRejectsUnsafeAppID(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	for _, bad := range []string{"", "ab", "Acme", "a-b_c!", "has space"} {
		if _, err := p.CreateAppDatabase(context.Background(), bad); err == nil {
			t.Fatalf("id %q must be rejected", bad)
		}
		if err := p.DeleteAppDatabase(context.Background(), bad); err == nil {
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
