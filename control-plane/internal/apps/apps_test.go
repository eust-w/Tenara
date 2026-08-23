package apps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dsn = "postgres://tenara:tenara_pg_dev@127.0.0.1:15432/tenara?sslmode=disable"

func newAppsStore(t *testing.T) (*Store, *pgxpool.Pool, int64) {
	pool, poolErr := pgxpool.New(context.Background(), dsn)
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	t.Cleanup(pool.Close)
	stamp := time.Now().UnixNano()
	return NewStore(pool), pool, stamp
}

func createOrgRow(t *testing.T, pool *pgxpool.Pool, stamp int64, tag string) string {
	t.Helper()
	var orgID string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`,
		fmt.Sprintf("qa-%s-%d", tag, stamp),
		fmt.Sprintf("qa-%s-%d", tag, stamp)).Scan(&orgID)
	if err != nil {
		t.Fatal(err)
	}
	return orgID
}

func TestCreateEnforcesFreeQuotaAndSoftDeleteFreesSlot(t *testing.T) {
	store, pool, stamp := newAppsStore(t)
	ctx := context.Background()
	orgA := createOrgRow(t, pool, stamp, "quota")

	ids := make([]string, 0, freeTierMaxApps)
	for i := range freeTierMaxApps {
		app, createErr := store.CreateApp(ctx, orgA,
			fmt.Sprintf("shop%d-%d", i, stamp), "")
		if createErr != nil {
			t.Fatalf("create %d = %v", i+1, createErr)
		}
		ids = append(ids, app.ID)
	}
	fourthName := fmt.Sprintf("over-%d", stamp)
	if _, fourthErr := store.CreateApp(ctx, orgA, fourthName, ""); !errors.Is(fourthErr, ErrQuotaExceeded) {
		t.Fatalf("4th = %v, want ErrQuotaExceeded", fourthErr)
	}

	dupName := fmt.Sprintf("shop0-%d", stamp)
	if _, dupErr := store.CreateApp(ctx, orgA, dupName, ""); !errors.Is(dupErr, ErrConflict) {
		t.Fatalf("duplicate = %v, want ErrConflict", dupErr)
	}

	if delErr := store.SoftDeleteApp(ctx, orgA, ids[2]); delErr != nil {
		t.Fatal(delErr)
	}
	list, listErr := store.ListApps(ctx, orgA)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(list) != 2 {
		t.Fatalf("list len %d after soft delete, want 2", len(list))
	}
	if _, thirdErr := store.CreateApp(ctx, orgA, fourthName, ""); thirdErr != nil {
		t.Fatalf("create after soft delete = %v, want success", thirdErr)
	}
}

func TestCrossOrgInvisible(t *testing.T) {
	store, pool, stamp := newAppsStore(t)
	ctx := context.Background()
	orgA := createOrgRow(t, pool, stamp, "iso-a")
	orgB := createOrgRow(t, pool, stamp, "iso-b")

	app, createErr := store.CreateApp(ctx, orgA, fmt.Sprintf("secret-%d", stamp), "")
	if createErr != nil {
		t.Fatal(createErr)
	}
	if _, getErr := store.GetApp(ctx, orgB, app.ID); !errors.Is(getErr, ErrNotFound) {
		t.Fatalf("cross-org get = %v, want ErrNotFound", getErr)
	}
	listB, listErr := store.ListApps(ctx, orgB)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(listB) != 0 {
		t.Fatalf("orgB sees %d apps", len(listB))
	}
}

func TestServicesAndEnvironments(t *testing.T) {
	store, pool, stamp := newAppsStore(t)
	ctx := context.Background()
	orgA := createOrgRow(t, pool, stamp, "svc")
	app, createErr := store.CreateApp(ctx, orgA, fmt.Sprintf("web%d", stamp), "")
	if createErr != nil {
		t.Fatal(createErr)
	}

	svc, addErr := store.AddService(ctx, orgA, app.ID, "web", "frontend", "node", 3000)
	if addErr != nil {
		t.Fatal(addErr)
	}
	if svc.Port != 3000 || svc.Type != "frontend" {
		t.Fatalf("svc = %+v", svc)
	}
	if _, dupErr := store.AddService(ctx, orgA, app.ID, "web", "backend", "go", 8000); !errors.Is(dupErr, ErrConflict) {
		t.Fatalf("duplicate service = %v, want ErrConflict", dupErr)
	}

	namespace, envErr := store.AddEnvironment(ctx, orgA, app.ID, "staging")
	if envErr != nil {
		t.Fatal(envErr)
	}
	shortID := strings.SplitN(app.ID, "-", 2)[0]
	if namespace != fmt.Sprintf("app-%s-staging", shortID) {
		t.Fatalf("namespace = %q", namespace)
	}
	var rowCount int
	if scanErr := pool.QueryRow(ctx,
		`SELECT count(*) FROM environments WHERE app_id = $1 AND name = 'staging'`,
		app.ID).Scan(&rowCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if rowCount != 1 {
		t.Fatal("environment row missing")
	}
	if _, dupEnv := store.AddEnvironment(ctx, orgA, app.ID, "staging"); !errors.Is(dupEnv, ErrConflict) {
		t.Fatalf("duplicate env = %v, want ErrConflict", dupEnv)
	}
}
