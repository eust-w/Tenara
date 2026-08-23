package apps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRevisionPersistenceAndRollbackTarget(t *testing.T) {
	store, pool, stamp := newAppsStore(t)
	ctx := context.Background()
	orgA := createOrgRow(t, pool, stamp, "rev")
	appID := createNamedAppRow(t, pool, orgA, fmt.Sprintf("revapp-%d", stamp))

	if _, envErr := store.EnsureEnvironment(ctx, orgA, appID, "production"); envErr != nil {
		t.Fatal(envErr)
	}
	depID := insertRunningDeployment(t, pool, ctx, store, orgA, appID, "production")

	t.Run("save revisions sequentially", func(t *testing.T) {
		runSaveRevisionsSequentially(t, store, ctx, orgA, appID, depID)
	})
	t.Run("rollback target selects second newest", func(t *testing.T) {
		runRollbackSecondNewest(t, store, ctx, orgA, appID, depID)
	})
	t.Run("single revision has no rollback target", func(t *testing.T) {
		runSingleRevisionNoRollback(t, store, pool, ctx, orgA, appID)
	})
	t.Run("missing digest rejected by schema constraint", func(t *testing.T) {
		runMissingDigestRejected(t, store, pool, ctx, orgA, appID, depID)
	})
}

func insertRunningDeployment(
	t *testing.T, pool *pgxpool.Pool, ctx context.Context, store *Store,
	orgID, appID, env string,
) string {
	t.Helper()
	if _, ensureErr := store.EnsureEnvironment(ctx, orgID, appID, env); ensureErr != nil {
		t.Fatal(ensureErr)
	}
	var depID string
	if scanErr := pool.QueryRow(ctx,
		`INSERT INTO deployments (app_id, environment_id, state)
		 SELECT $1, e.id, 'RUNNING' FROM environments e
		 WHERE e.app_id = $1 AND e.name = $2
		 RETURNING id::text`, appID, env).Scan(&depID); scanErr != nil {
		t.Fatal(scanErr)
	}
	return depID
}

func runSaveRevisionsSequentially(
	t *testing.T, store *Store, ctx context.Context, orgA, appID, depID string,
) {
	t.Helper()
	for i, ch := range []string{"a", "b", "c"} {
		r, saveErr := store.SaveRevision(ctx, orgA, appID, depID,
			RevisionInput{
				GitSHA:      fmt.Sprintf("sha%d", i+1),
				ImageDigest: "sha256:" + strings.Repeat(ch, 64),
			})
		if saveErr != nil {
			t.Fatalf("save %d = %v", i+1, saveErr)
		}
		if r.Revision != i+1 {
			t.Fatalf("revision = %d, want %d", r.Revision, i+1)
		}
	}
}

func runRollbackSecondNewest(
	t *testing.T, store *Store, ctx context.Context, orgA, appID, depID string,
) {
	t.Helper()
	target, targetErr := store.RollbackTarget(ctx, orgA, appID, depID)
	if targetErr != nil {
		t.Fatal(targetErr)
	}
	want := "sha256:" + strings.Repeat("b", 64)
	if target.ImageDigest != want {
		t.Fatalf("rollback digest = %s, want %s", target.ImageDigest, want)
	}
	if target.Revision != 2 {
		t.Fatalf("rollback revision = %d, want 2", target.Revision)
	}
}

func runSingleRevisionNoRollback(
	t *testing.T, store *Store, pool *pgxpool.Pool,
	ctx context.Context, orgA, appID string,
) {
	t.Helper()
	var dep2 string
	firstErr := pool.QueryRow(ctx,
		insertStagingSQL(), appID).Scan(&dep2)
	if firstErr != nil {
		if _, ensureErr := store.EnsureEnvironment(ctx, orgA, appID, "staging"); ensureErr != nil {
			t.Fatal(ensureErr)
		}
		if scanErr := pool.QueryRow(ctx,
			insertStagingSQL(), appID).Scan(&dep2); scanErr != nil {
			t.Fatal(scanErr)
		}
	}
	digest := "sha256:" + strings.Repeat("d", 64)
	if _, saveErr := store.SaveRevision(ctx, orgA, appID, dep2,
		RevisionInput{ImageDigest: digest}); saveErr != nil {
		t.Fatal(saveErr)
	}
	if _, targetErr := store.RollbackTarget(ctx, orgA, appID, dep2); !errors.Is(targetErr, ErrNotEnoughRevisions) {
		t.Fatalf("single-revision rollback = %v, want ErrNotEnoughRevisions", targetErr)
	}
}

func runMissingDigestRejected(
	t *testing.T, store *Store, pool *pgxpool.Pool,
	ctx context.Context, orgA, appID, depID string,
) {
	t.Helper()
	if _, rawErr := pool.Exec(ctx,
		`INSERT INTO deployment_revisions
		 (deployment_id, revision, image_digest) VALUES ($1, 99, '')`,
		depID); rawErr == nil {
		t.Fatal("empty digest must be rejected by CHECK constraint")
	}
	if _, typedErr := store.SaveRevision(ctx, orgA, appID, depID,
		RevisionInput{ImageDigest: "not-a-digest"}); !errors.Is(typedErr, ErrInvalidDigest) {
		t.Fatalf("typed error = %v, want ErrInvalidDigest", typedErr)
	}
}

func insertStagingSQL() string {
	return `INSERT INTO deployments (app_id, environment_id, state)
	 SELECT $1, e.id, 'RUNNING' FROM environments e
	 WHERE e.app_id = $1 AND e.name = 'staging'
	 RETURNING id::text`
}
