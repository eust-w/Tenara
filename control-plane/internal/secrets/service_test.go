package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/kms"
)

const (
	testDSN      = "postgres://tenara:tenara_pg_dev@127.0.0.1:15432/tenara?sslmode=disable"
	masterKeyHex = "abababababababababababababababababababababababababababababababab"
	otherKeyHex  = "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
)

func newSecretsHarness(t *testing.T) (*Service, *pgxpool.Pool, *kms.Stub, func() string) {
	t.Helper()
	pool, poolErr := pgxpool.New(context.Background(), testDSN)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)

	kmsStub, stubErr := kms.NewStub(masterKeyHex)
	if stubErr != nil {
		t.Fatal(stubErr)
	}
	stamp := time.Now().UnixNano()
	var orgID string
	if orgErr := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`,
		fmt.Sprintf("sec-org-%d", stamp),
		fmt.Sprintf("sec-org-%d", stamp)).Scan(&orgID); orgErr != nil {
		t.Fatal(orgErr)
	}
	var seq int64
	makeApp := func() string {
		seq++
		var appID string
		if appErr := pool.QueryRow(context.Background(),
			`INSERT INTO applications (org_id, name, slug) VALUES ($1,$2,$2) RETURNING id`,
			orgID, fmt.Sprintf("secapp-%d-%d", stamp, seq)).Scan(&appID); appErr != nil {
			t.Fatal(appErr)
		}
		return appID
	}
	return New(pool, kmsStub), pool, kmsStub, makeApp
}

func TestSetThenListMasksPlaintext(t *testing.T) {
	svc, _, _, makeApp := newSecretsHarness(t)
	ctx := context.Background()
	appID := makeApp()

	for _, pair := range [][2]string{
		{"DATABASE_URL", "postgres://real-secret-db"},
		{"API_KEY", "sk-real-secret-key"},
	} {
		if setErr := svc.SetSecret(ctx, appID, pair[0], pair[1]); setErr != nil {
			t.Fatal(setErr)
		}
	}

	items, listErr := svc.ListSecrets(ctx, appID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	raw, marshalErr := json.Marshal(items)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	body := string(raw)
	if strings.Count(body, "configured") != 2 {
		t.Fatalf("expected two masked values: %s", body)
	}
	if strings.Contains(body, "postgres://real-secret-db") ||
		strings.Contains(body, "sk-real-secret-key") {
		t.Fatal("plaintext leaked through masked listing")
	}
}

func TestRevealRoundTrip(t *testing.T) {
	svc, _, _, makeApp := newSecretsHarness(t)
	ctx := context.Background()
	appID := makeApp()

	const val = "postgres://round-trip-value"
	if setErr := svc.SetSecret(ctx, appID, "DATABASE_URL", val); setErr != nil {
		t.Fatal(setErr)
	}
	got, revealErr := svc.Reveal(ctx, appID, "DATABASE_URL")
	if revealErr != nil {
		t.Fatal(revealErr)
	}
	if got != val {
		t.Fatalf("revealed %q, want %q", got, val)
	}
}

// Acceptance core: both stored revision ciphertexts must decrypt with the
// CURRENT master key back to their original plaintexts.
func TestRevisionChainKeepsHistory(t *testing.T) {
	svc, pool, kmsStub, makeApp := newSecretsHarness(t)
	ctx := context.Background()
	appID := makeApp()
	const name = "API_KEY"

	if setErr := svc.SetSecret(ctx, appID, name, "value-v1"); setErr != nil {
		t.Fatal(setErr)
	}
	if setErr := svc.SetSecret(ctx, appID, name, "value-v2"); setErr != nil {
		t.Fatal(setErr)
	}

	rows, queryErr := pool.Query(ctx,
		`SELECT sr.ciphertext, sr.version
		 FROM secret_revisions sr JOIN secrets s ON s.id = sr.secret_id
		 WHERE s.app_id = $1 AND s.name = $2 ORDER BY sr.version`, appID, name)
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	defer rows.Close()

	type rev struct {
		ciphertext []byte
		version    int
	}
	var revs []rev
	for rows.Next() {
		var r rev
		if scanErr := rows.Scan(&r.ciphertext, &r.version); scanErr != nil {
			t.Fatal(scanErr)
		}
		revs = append(revs, r)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	if len(revs) != 2 {
		t.Fatalf("revisions = %d, want 2", len(revs))
	}

	want := map[int]string{1: "value-v1", 2: "value-v2"}
	for _, r := range revs {
		plain, decErr := kmsStub.Decrypt(r.ciphertext)
		if decErr != nil {
			t.Fatalf("decrypt revision %d: %v", r.version, decErr)
		}
		if string(plain) != want[r.version] {
			t.Fatalf("revision %d = %q, want %q", r.version, plain, want[r.version])
		}
	}
}

func TestWrongMasterKeyFailsClosed(t *testing.T) {
	svc, pool, _, makeApp := newSecretsHarness(t)
	ctx := context.Background()
	appID := makeApp()
	if setErr := svc.SetSecret(ctx, appID, "K", "some-value"); setErr != nil {
		t.Fatal(setErr)
	}

	badStub, stubErr := kms.NewStub(otherKeyHex)
	if stubErr != nil {
		t.Fatal(stubErr)
	}
	badSvc := New(pool, badStub)
	if _, revealErr := badSvc.Reveal(ctx, appID, "K"); revealErr == nil {
		t.Fatal("wrong master key must fail decryption")
	}
}
