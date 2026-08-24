package apps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Binding kinds accepted by database.create (RB§19): mongo plus the P2
// product face for cache and object storage (todo89, D2-P2-2).
const (
	KindMongo   = "mongodb"
	KindRedis   = "redis"
	KindStorage = "storage"
)

// ErrUnsupportedKind rejects binding kinds outside the supported set.
var ErrUnsupportedKind = errors.New("unsupported database kind")

func normalizeKind(kind string) (string, error) {
	switch kind {
	case "":
		return KindMongo, nil
	case KindMongo, KindRedis, KindStorage:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedKind, kind)
	}
}

// RequestDatabase records the desired binding (RB-19) without touching any
// real datastore: provisioning is delegated to the data-plane controllers.
// Repeat requests for the same kind merge idempotently; different kinds own
// independent rows.
func (s *Store) RequestDatabase(
	ctx context.Context, orgID, appID, kind, isolation string,
) (DatabaseRow, BindingRow, error) {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return DatabaseRow{}, BindingRow{}, getAppErr
	}
	normalized, kindErr := normalizeKind(kind)
	if kindErr != nil {
		return DatabaseRow{}, BindingRow{}, kindErr
	}
	if isolation != "shared" && isolation != "dedicated" {
		isolation = "shared"
	}

	var db DatabaseRow
	selectErr := s.pool.QueryRow(ctx,
		`SELECT id::text, state FROM databases WHERE app_id = $1 AND type = $2`,
		appID, normalized).Scan(&db.ID, &db.State)
	switch {
	case errors.Is(selectErr, pgx.ErrNoRows):
		insertErr := s.pool.QueryRow(ctx,
			`INSERT INTO databases (app_id, type, isolation)
			 VALUES ($1, $2, $3)
			 RETURNING id::text, app_id::text, type, isolation, state`,
			appID, normalized, isolation).
			Scan(&db.ID, &db.AppID, &db.Type, &db.Isolation, &db.State)
		if insertErr != nil {
			return DatabaseRow{}, BindingRow{}, insertErr
		}
		if _, bindErr := s.pool.Exec(ctx,
			`INSERT INTO database_bindings (database_id, app_id, state)
			 VALUES ($1, $2, 'pending')`,
			db.ID, appID); bindErr != nil {
			return DatabaseRow{}, BindingRow{}, bindErr
		}
		return db, BindingRow{State: "pending"}, nil
	case selectErr != nil:
		return DatabaseRow{}, BindingRow{}, selectErr
	}

	if _, updateErr := s.pool.Exec(ctx,
		`UPDATE databases SET isolation = $2 WHERE id = $1`, db.ID, isolation); updateErr != nil {
		return DatabaseRow{}, BindingRow{}, updateErr
	}
	db.Isolation = isolation
	binding, bindErr := s.bindingFor(ctx, db.ID)
	if bindErr != nil {
		return DatabaseRow{}, BindingRow{}, bindErr
	}
	return db, binding, nil
}

func (s *Store) bindingFor(ctx context.Context, databaseID string) (BindingRow, error) {
	var b BindingRow
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, COALESCE(credential_secret_ref,''), state
		 FROM database_bindings WHERE database_id = $1`, databaseID).
		Scan(&b.ID, &b.CredentialRef, &b.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return BindingRow{}, ErrNotFound
	}
	return b, err
}

// MarkBindingReady simulates the database-controller completion callback
// (todo49 replaces this stub with the real reconcile loop).
func (s *Store) MarkBindingReady(ctx context.Context, orgID, appID string) error {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return getAppErr
	}
	shortID := strings.SplitN(appID, "-", 2)[0]
	dbName := "app_" + shortID
	if _, execErr := s.pool.Exec(ctx,
		`UPDATE database_bindings SET state = 'ready'
		 WHERE app_id = $1 AND state = 'pending'`, appID); execErr != nil {
		return execErr
	}
	_, execErr := s.pool.Exec(ctx,
		`UPDATE databases SET state = 'ready', db_name = $2
		 WHERE app_id = $1 AND state = 'pending'`,
		appID, dbName)
	return execErr
}

type DatabaseRow struct {
	ID        string `json:"id"`
	AppID     string `json:"app_id"`
	Type      string `json:"type"`
	Isolation string `json:"isolation"`
	State     string `json:"state"`
}

type BindingRow struct {
	ID            string `json:"id"`
	CredentialRef string `json:"credential_secret_ref,omitempty"`
	State         string `json:"state"`
}
