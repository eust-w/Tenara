package local

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"tenara/providers/types"
)

const (
	testAlias = "mcalias"
	stubPW    = "abcdef0123456789abcdef0123456789"
)

type fakeRunner struct {
	respErr      error
	gotArgs      [][]string
	stagedPolicy string
}

func (f *fakeRunner) Run(_ context.Context, _ *Config, args []string) (string, error) {
	f.gotArgs = append(f.gotArgs, args)
	for _, a := range args {
		if strings.HasSuffix(a, ".json") {
			if body, rErr := os.ReadFile(a); rErr == nil {
				f.stagedPolicy = string(body)
			}
		}
	}
	return "", f.respErr
}

func newTestProvider(f *fakeRunner) *Provider {
	return NewWith(Config{Alias: testAlias}, f, func() (string, error) { return stubPW, nil })
}

func TestBuildPolicyJSONScopedPrefix(t *testing.T) {
	body := buildPolicyJSON("acme")
	for _, want := range []string{
		`arn:aws:s3:::` + bucketName + `/app_acme/*`,
		`arn:aws:s3:::` + bucketName,
		`"s3:prefix": "app_acme/*"`,
		`"s3:ListBucket"`,
		`"s3:GetObject"`, `"s3:PutObject"`, `"s3:DeleteObject"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("policy missing %s:\n%s", want, body)
		}
	}
	for _, banned := range []string{
		`arn:aws:s3:::` + bucketName + `/*`,
		`"Resource": "*"`,
		`"Action": "s3:*"`,
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("policy contains bucket-level wildcard %s:\n%s", banned, body)
		}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("policy must be valid json: %v", err)
	}
}

func TestCreateAppStorageSequence(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	cred, err := p.CreateAppStorage(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 3 {
		t.Fatalf("steps = %d, want user-add/policy-add/attach", len(f.gotArgs))
	}

	userAdd := f.gotArgs[0]
	wantUA := []string{"admin", "user", "add", testAlias, "app_acme", stubPW}
	for i, w := range wantUA {
		if userAdd[i] != w {
			t.Fatalf("user add arg %d = %q, want %q (%v)", i, userAdd[i], w, userAdd)
		}
	}

	policyAdd := f.gotArgs[1]
	if len(policyAdd) != 6 || policyAdd[4] != "app_acme" {
		t.Fatalf("policy add = %v", policyAdd)
	}
	if f.stagedPolicy != buildPolicyJSON("acme") {
		t.Fatalf("staged policy diverges from template:\n%s", f.stagedPolicy)
	}

	attach := f.gotArgs[2]
	wantAttach := []string{"admin", "policy", "attach", testAlias, "app_acme", "--user", "app_acme"}
	for i, w := range wantAttach {
		if attach[i] != w {
			t.Fatalf("attach arg %d = %q, want %q (%v)", i, attach[i], w, attach)
		}
	}

	if cred.Username != "app_acme" || cred.Password != stubPW {
		t.Fatalf("credential = %+v", cred)
	}
	if cred.URI != "s3://"+bucketName+"/app_acme" {
		t.Fatalf("uri = %s", cred.URI)
	}
}

func TestDeleteAppStorageDetachesAndRemoves(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	if err := p.DeleteAppStorage(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
	if len(f.gotArgs) != 2 {
		t.Fatalf("steps = %d, want detach then user-remove", len(f.gotArgs))
	}
	detach := f.gotArgs[0]
	wantD := []string{"admin", "policy", "detach", testAlias, "app_acme", "--user", "app_acme"}
	for i, w := range wantD {
		if detach[i] != w {
			t.Fatalf("detach arg %d = %q, want %q", i, detach[i], w)
		}
	}
	remove := f.gotArgs[1]
	wantR := []string{"admin", "user", "remove", testAlias, "app_acme"}
	for i, w := range wantR {
		if remove[i] != w {
			t.Fatalf("remove arg %d = %q, want %q", i, remove[i], w)
		}
	}
}

func TestEnsureBucketTargetsTenantBucket(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	if err := p.EnsureBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := f.gotArgs[0]
	if len(got) != 2 || got[0] != "mb" || got[1] != testAlias+"/"+bucketName {
		t.Fatalf("mb args = %v", got)
	}
}

func TestRejectsUnsafeAppID(t *testing.T) {
	f := &fakeRunner{}
	p := newTestProvider(f)

	for _, bad := range []string{"", "ab", "Acme", "a-b_c!", "has space"} {
		if _, err := p.CreateAppStorage(context.Background(), bad); err == nil {
			t.Fatalf("id %q must be rejected", bad)
		}
		if err := p.DeleteAppStorage(context.Background(), bad); err == nil {
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
