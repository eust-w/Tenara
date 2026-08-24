package apps

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func createNamedAppRow(t *testing.T, pool *pgxpool.Pool, orgID, name string) string {
	t.Helper()
	var appID string
	if appErr := pool.QueryRow(context.Background(),
		`INSERT INTO applications (org_id, name, slug) VALUES ($1,$2,$2) RETURNING id`,
		orgID, name).Scan(&appID); appErr != nil {
		t.Fatal(appErr)
	}
	return appID
}

func TestDatabaseRequestLifecycle(t *testing.T) {
	store, pool, stamp := newAppsStore(t)
	ctx := context.Background()
	orgA := createOrgRow(t, pool, stamp, "db")
	appID := createNamedAppRow(t, pool, orgA, fmt.Sprintf("dbapp-%d", stamp))

	t.Run("create lands pending rows", func(t *testing.T) {
		runCreatePending(t, store, ctx, orgA, appID)
	})
	t.Run("repeat request merges idempotently", func(t *testing.T) {
		runRepeatMerge(t, store, pool, ctx, orgA, appID)
	})
	t.Run("stub controller callback flips ready", func(t *testing.T) {
		runStubCallbackReady(t, store, pool, ctx, orgA, appID)
	})
	t.Run("isolation upgrade merges on existing row", func(t *testing.T) {
		db, _, reqErr := store.RequestDatabase(ctx, orgA, appID, "mongodb", "dedicated")
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		if db.Isolation != "dedicated" {
			t.Fatalf("isolation = %s, want dedicated", db.Isolation)
		}
	})
}

func runCreatePending(
	t *testing.T, store *Store, ctx context.Context, orgA, appID string,
) {
	t.Helper()
	db, binding, createErr := store.RequestDatabase(ctx, orgA, appID, "mongodb", "shared")
	if createErr != nil {
		t.Fatal(createErr)
	}
	if db.State != "pending" || binding.State != "pending" {
		t.Fatalf("states = %s/%s, want pending/pending", db.State, binding.State)
	}
	if db.Isolation != "shared" {
		t.Fatalf("isolation = %s, want shared", db.Isolation)
	}
}

func runRepeatMerge(
	t *testing.T, store *Store, pool *pgxpool.Pool,
	ctx context.Context, orgA, appID string,
) {
	t.Helper()
	db2, b2, reqErr := store.RequestDatabase(ctx, orgA, appID, "mongodb", "shared")
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	var dbCount, bindCount int
	if scanErr := pool.QueryRow(ctx,
		`SELECT count(*) FROM databases WHERE app_id = $1`, appID).Scan(&dbCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := pool.QueryRow(ctx,
		`SELECT count(*) FROM database_bindings WHERE app_id = $1`, appID).Scan(&bindCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if dbCount != 1 || bindCount != 1 {
		t.Fatalf("rows %d/%d, want 1/1", dbCount, bindCount)
	}
	if db2.State != "pending" || b2.State != "pending" {
		t.Fatalf("merged states = %s/%s", db2.State, b2.State)
	}
}

func runStubCallbackReady(
	t *testing.T, store *Store, pool *pgxpool.Pool,
	ctx context.Context, orgA, appID string,
) {
	t.Helper()
	if readyErr := store.MarkBindingReady(ctx, orgA, appID); readyErr != nil {
		t.Fatal(readyErr)
	}
	var dbState, dbName, bindState string
	if scanErr := pool.QueryRow(ctx,
		`SELECT state, COALESCE(db_name,'') FROM databases
		 WHERE app_id = $1 AND type = 'mongodb'`, appID).
		Scan(&dbState, &dbName); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := pool.QueryRow(ctx,
		`SELECT state FROM database_bindings WHERE app_id = $1 LIMIT 1`,
		appID).Scan(&bindState); scanErr != nil {
		t.Fatal(scanErr)
	}
	if dbState != "ready" || bindState != "ready" {
		t.Fatalf("states after callback = %s/%s, want ready/ready", dbState, bindState)
	}
	if !strings.HasPrefix(dbName, "app_") {
		t.Fatalf("db_name = %q", dbName)
	}
}
