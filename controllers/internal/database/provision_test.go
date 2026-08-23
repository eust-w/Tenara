package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tenara/controllers/internal/appenv"
	"tenara/providers/types"
)

const testURI = "mongodb://app_acme:s3cret@127.0.0.1:27017/app_acme?authSource=app_acme"

type fakeDB struct {
	cred  *types.Credential
	err   error
	calls int
}

func (f *fakeDB) CreateAppDatabase(context.Context, string) (*types.Credential, error) {
	f.calls++
	return f.cred, f.err
}
func (f *fakeDB) DeleteAppDatabase(context.Context, string) error { return nil }
func (f *fakeDB) Healthz(context.Context) error                   { return nil }

type fakeCache struct{ calls int }

func (f *fakeCache) CreateAppCache(context.Context, string) (*types.Credential, error) {
	f.calls++
	return &types.Credential{Username: "cache_acme", URI: "redis://cache_acme:pw@h:6379"}, nil
}
func (f *fakeCache) DeleteAppCache(context.Context, string) error { return nil }
func (f *fakeCache) Healthz(context.Context) error                { return nil }

type fakeStorage struct{ calls int }

func (f *fakeStorage) CreateAppStorage(context.Context, string) (*types.Credential, error) {
	f.calls++
	return &types.Credential{Username: "app_acme", URI: "s3://tenara-tenant/app_acme"}, nil
}
func (f *fakeStorage) DeleteAppStorage(context.Context, string) error { return nil }
func (f *fakeStorage) Healthz(context.Context) error                  { return nil }

// fakeSecrets tags sealed values so tests can prove encryption ran before any
// namespace injection reached the recorded payload.
type fakeSecrets struct {
	encErr     error
	injections []injection
	encCalls   int
}

type injection struct {
	data      map[string][]byte
	namespace string
}

func (f *fakeSecrets) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	f.encCalls++
	if f.encErr != nil {
		return nil, f.encErr
	}
	return append([]byte("sealed:"), plaintext...), nil
}

func (f *fakeSecrets) Decrypt(_ context.Context, sealed []byte) ([]byte, error) {
	return []byte(strings.TrimPrefix(string(sealed), "sealed:")), nil
}

func (f *fakeSecrets) InjectToNamespace(_ context.Context, _, namespace string, secrets map[string][]byte) error {
	cp := make(map[string][]byte, len(secrets))
	for k, v := range secrets {
		cp[k] = append([]byte(nil), v...)
	}
	f.injections = append(f.injections, injection{namespace: namespace, data: cp})
	return nil
}

func happyDeps(db *fakeDB, sec *fakeSecrets) Deps {
	return Deps{Databases: db, Caches: &fakeCache{}, Storages: &fakeStorage{}, Secrets: sec}
}

func mongoSpec() DatabaseBindingSpec {
	return DatabaseBindingSpec{AppID: "acme", Env: "prod", Kind: KindMongo}
}

func TestProvisionMongoHappyPathSealsBeforeInject(t *testing.T) {
	db := &fakeDB{cred: &types.Credential{Username: "app_acme", URI: testURI}}
	sec := &fakeSecrets{}

	out, err := ProvisionBinding(context.Background(), happyDeps(db, sec), mongoSpec())
	if err != nil {
		t.Fatal(err)
	}

	if db.calls != 1 || sec.encCalls != 1 {
		t.Fatalf("calls db=%d enc=%d", db.calls, sec.encCalls)
	}
	if len(sec.injections) != 1 {
		t.Fatalf("injections = %d", len(sec.injections))
	}

	wantNS := appenv.NamespaceName("acme", "prod")
	ij := sec.injections[0]
	if ij.namespace != wantNS {
		t.Fatalf("namespace = %q, want %q", ij.namespace, wantNS)
	}
	got := string(ij.data[keyMongoURI])
	if got != "sealed:"+testURI {
		t.Fatalf("injected value bypassed sealing flow: %q", got)
	}
	if out.SecretName != secretName("acme") || out.SecretKey != keyMongoURI || out.Namespace != wantNS {
		t.Fatalf("out = %+v", out)
	}
}

func TestKindDispatchAndKeys(t *testing.T) {
	db := &fakeDB{cred: &types.Credential{URI: "mongo://x"}}
	cache := &fakeCache{}
	sto := &fakeStorage{}
	sec := &fakeSecrets{}
	deps := Deps{Databases: db, Caches: cache, Storages: sto, Secrets: sec}

	cases := []struct {
		kind BindingKind
		key  string
	}{
		{KindMongo, keyMongoURI},
		{KindRedis, keyRedisURI},
		{KindStorage, keyS3Target},
	}
	for _, tc := range cases {
		spec := DatabaseBindingSpec{AppID: "acme", Env: "dev", Kind: tc.kind}
		out, err := ProvisionBinding(context.Background(), deps, spec)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if out.SecretKey != tc.key {
			t.Fatalf("%s key = %s, want %s", tc.kind, out.SecretKey, tc.key)
		}
	}
	if db.calls != 1 || cache.calls != 1 || sto.calls != 1 {
		t.Fatalf("dispatch counts db=%d cache=%d storage=%d", db.calls, cache.calls, sto.calls)
	}
}

func TestUnknownKindFailsClosed(t *testing.T) {
	db := &fakeDB{cred: &types.Credential{URI: "x"}}
	sec := &fakeSecrets{}

	spec := DatabaseBindingSpec{AppID: "acme", Env: "prod", Kind: "oracle"}
	if _, err := ProvisionBinding(context.Background(), happyDeps(db, sec), spec); err == nil {
		t.Fatal("unknown kind must fail closed")
	}
	if db.calls != 0 || sec.encCalls != 0 || len(sec.injections) != 0 {
		t.Fatal("unknown kind must produce zero side effects")
	}
}

func TestProviderFailurePropagatesForRetry(t *testing.T) {
	db := &fakeDB{err: errors.New("backend down")}
	sec := &fakeSecrets{}

	if _, err := ProvisionBinding(context.Background(), happyDeps(db, sec), mongoSpec()); err == nil {
		t.Fatal("provider failure must propagate for backoff retry")
	}
	if sec.encCalls != 0 || len(sec.injections) != 0 {
		t.Fatal("no sealing or injection may happen after provider failure")
	}
}

func TestEncryptFailurePropagates(t *testing.T) {
	db := &fakeDB{cred: &types.Credential{URI: testURI}}
	sec := &fakeSecrets{encErr: errors.New("kms down")}

	if _, err := ProvisionBinding(context.Background(), happyDeps(db, sec), mongoSpec()); err == nil {
		t.Fatal("encrypt failure must propagate")
	}
	if len(sec.injections) != 0 {
		t.Fatal("unsealed values must never reach injection")
	}
}
