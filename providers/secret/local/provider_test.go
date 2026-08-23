package local

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"tenara/providers/types"
)

func testKey(base byte) []byte {
	key := make([]byte, aes256KeySize)
	for i := range key {
		key[i] = base + byte(i)
	}
	return key
}

func mustSealer(t *testing.T, base byte) *Sealer {
	t.Helper()
	s, err := NewSealer(testKey(base))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var _ types.SecretProvider = (*Provider)(nil)

func sealValue(t *testing.T, s *Sealer, val string) []byte {
	t.Helper()
	blob, err := s.Encrypt([]byte(val))
	if err != nil {
		t.Fatal(err)
	}
	return blob
}

func TestSealRoundTripAndVersionHeader(t *testing.T) {
	s := mustSealer(t, 0x10)
	plain := []byte("hello tenara")

	sealed, err := s.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if sealed[0] != sealVersionV1 {
		t.Fatalf("version header = %#x", sealed[0])
	}
	if len(sealed) <= 1+gcmNonceSize {
		t.Fatal("ciphertext body missing")
	}

	got, err := s.Decrypt(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip = %q, want %q", got, plain)
	}
}

func TestWrongKeyFailsWithSentinel(t *testing.T) {
	a := mustSealer(t, 0x10)
	b := mustSealer(t, 0x80)

	sealed, sErr := a.Encrypt([]byte("payload"))
	if sErr != nil {
		t.Fatal(sErr)
	}
	if _, dErr := b.Decrypt(sealed); !errors.Is(dErr, ErrSealMismatch) {
		t.Fatalf("want ErrSealMismatch, got %v", dErr)
	}
}

func TestRejectsBadMasterKeyLength(t *testing.T) {
	if _, err := NewSealer(make([]byte, 16)); err == nil {
		t.Fatal("short key must be rejected")
	}
	if _, err := NewSealer(nil); err == nil {
		t.Fatal("nil key must be rejected")
	}
}

func TestInjectWritesPlaintextIntoTargetOnly(t *testing.T) {
	s := mustSealer(t, 0x20)
	cs := fake.NewSimpleClientset()
	p := &Provider{sealer: s, clientset: cs}

	plain := map[string]string{"MONGO_URI": "mongodb://x", "REDIS_URI": "redis://y"}
	sealed := map[string][]byte{
		"MONGO_URI": sealValue(t, s, plain["MONGO_URI"]),
		"REDIS_URI": sealValue(t, s, plain["REDIS_URI"]),
	}

	target := "app-acme-prod"
	if err := p.InjectToNamespace(context.Background(), "acme", target, sealed); err != nil {
		t.Fatal(err)
	}

	got, gErr := cs.CoreV1().Secrets(target).Get(
		context.Background(), SecretName("acme"), metav1.GetOptions{})
	if gErr != nil {
		t.Fatal(gErr)
	}
	if len(got.Data) != len(plain) {
		t.Fatalf("data keys = %d, want %d", len(got.Data), len(plain))
	}
	for k, want := range plain {
		if string(got.Data[k]) != want {
			t.Fatalf("data[%s] = %q, want %q", k, got.Data[k], want)
		}
	}
	if got.Labels[labelAppID] != "acme" || got.Labels[labelManagedBy] != labelManagedVal {
		t.Fatalf("labels = %v", got.Labels)
	}

	for _, action := range cs.Actions() {
		ns := action.GetNamespace()
		if ns != "" && ns != target && strings.Contains(action.GetResource().Resource, "secret") {
			t.Fatalf("secret write leaked into %s (%s)", ns, action.GetVerb())
		}
	}
}

func TestInjectUpsertsExistingSecret(t *testing.T) {
	s := mustSealer(t, 0x20)
	cs := fake.NewSimpleClientset()
	p := &Provider{sealer: s, clientset: cs}

	first := map[string][]byte{"K": sealValue(t, s, "v1")}
	target := "app-acme-prod"
	if err := p.InjectToNamespace(context.Background(), "acme", target, first); err != nil {
		t.Fatal(err)
	}
	second := map[string][]byte{"K": sealValue(t, s, "v2")}
	if err := p.InjectToNamespace(context.Background(), "acme", target, second); err != nil {
		t.Fatal(err)
	}

	got, gErr := cs.CoreV1().Secrets(target).Get(
		context.Background(), SecretName("acme"), metav1.GetOptions{})
	if gErr != nil {
		t.Fatal(gErr)
	}
	if string(got.Data["K"]) != "v2" {
		t.Fatalf("upsert failed: data[K] = %q", got.Data["K"])
	}
	if _, ok := got.Data["stale"]; ok {
		t.Fatal("stale keys must be replaced wholesale")
	}
}

func TestInjectDecryptFailurePropagatesWithoutWrites(t *testing.T) {
	wrong := mustSealer(t, 0x99)
	cs := fake.NewSimpleClientset()
	p := &Provider{sealer: mustSealer(t, 0x20), clientset: cs}

	badBlob, xErr := wrong.Encrypt([]byte("z"))
	if xErr != nil {
		t.Fatal(xErr)
	}
	err := p.InjectToNamespace(context.Background(), "acme", "ns-a", map[string][]byte{"K": badBlob})
	if !errors.Is(err, ErrSealMismatch) {
		t.Fatalf("want ErrSealMismatch, got %v", err)
	}
	if len(cs.Actions()) != 0 {
		t.Fatalf("failed decrypt must issue zero writes, got %d", len(cs.Actions()))
	}
}

func TestReEncryptRotatesAllEntries(t *testing.T) {
	old := mustSealer(t, 0x10)
	fresh := mustSealer(t, 0xF0)
	p := &Provider{sealer: old, clientset: fake.NewSimpleClientset()}

	src := map[string][]byte{
		"a": sealValue(t, old, "one"),
		"b": sealValue(t, old, "two"),
		"c": sealValue(t, old, "three"),
	}

	out, rErr := p.ReEncrypt(src, fresh)
	if rErr != nil {
		t.Fatal(rErr)
	}
	if len(out) != len(src) {
		t.Fatalf("rotated entries = %d, want %d", len(out), len(src))
	}
	for key, want := range map[string]string{"a": "one", "b": "two", "c": "three"} {
		plain, dErr := fresh.Decrypt(out[key])
		if dErr != nil {
			t.Fatalf("fresh decrypt %s: %v", key, dErr)
		}
		if string(plain) != want {
			t.Fatalf("rotated %s = %q, want %q", key, plain, want)
		}
		if _, legacyErr := old.Decrypt(out[key]); !errors.Is(legacyErr, ErrSealMismatch) {
			t.Fatalf("old key must fail on rotated %s: %v", key, legacyErr)
		}
	}
}

func TestSecretTypeAndNameContract(t *testing.T) {
	if SecretName("acme") != "app-acme-secrets" {
		t.Fatalf("name = %s", SecretName("acme"))
	}
	s := mustSealer(t, 0x30)
	cs := fake.NewSimpleClientset()
	p := &Provider{sealer: s, clientset: cs}
	if err := p.InjectToNamespace(context.Background(), "acme", "ns", map[string][]byte{
		"K": sealValue(t, s, "v"),
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := cs.CoreV1().Secrets("ns").Get(
		context.Background(), SecretName("acme"), metav1.GetOptions{})
	if got.Type != corev1.SecretTypeOpaque {
		t.Fatalf("type = %s", got.Type)
	}
}
