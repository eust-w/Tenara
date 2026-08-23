// Package apps implements application/service/environment management
// (plan tenara-agent-paas#16, RB-5 RB-29 R10). Storage only: no Kubernetes
// resources are touched here, rendering belongs to the controllers.
package apps

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// freeTierMaxApps pins the R10 org-level quota; namespace-level ResourceQuota
// is layered separately by the tenant controller (todo37).
const freeTierMaxApps = 3

var (
	ErrNotFound      = errors.New("not found")
	ErrQuotaExceeded = errors.New("quota exceeded")
	ErrConflict      = errors.New("conflict")
)

type App struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt string
}

func slugify(name string) string {
	lower := strings.ToLower(name)
	b := make([]byte, 0, len(lower))
	for _, r := range lower {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b = append(b, byte(r)) //nolint:gosec // branch proves r is ASCII [0-9a-z]
		default:
			b = append(b, '-')
		}
	}
	return strings.Trim(string(b), "-")
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func validUUID(id string) bool {
	_, parseErr := uuid.Parse(id)
	return parseErr == nil
}

// CreateApp enforces the org-level quota atomically: the guard lives inside
// the single INSERT..SELECT statement, so racing creations cannot both slip
// under the limit.
func (s *Store) CreateApp(ctx context.Context, orgID, name, createdBy string) (App, error) {
	slug := slugify(name)
	if slug == "" || strings.TrimSpace(name) == "" {
		return App{}, fmt.Errorf("%w: name needs alphanumeric characters", ErrConflict)
	}
	var app App
	insertErr := s.pool.QueryRow(ctx, `
INSERT INTO applications (org_id, name, slug, created_by)
SELECT $1, $2, $3, NULLIF($4,'')::uuid
WHERE (SELECT count(*) FROM applications WHERE org_id = $1 AND deleted_at IS NULL) < $5
ON CONFLICT (org_id, name) DO NOTHING
RETURNING id, name, slug, created_at::text`,
		orgID, name, slug, createdBy, freeTierMaxApps).
		Scan(&app.ID, &app.Name, &app.Slug, &app.CreatedAt)
	if errors.Is(insertErr, pgx.ErrNoRows) {
		return App{}, s.classifyCreateRejection(ctx, orgID, name)
	}
	if insertErr != nil {
		return App{}, insertErr
	}
	return app, nil
}

// GetApp scopes lookups to the caller organization: cross-org access and
// missing rows are indistinguishable by design.
// classifyCreateRejection explains why an insert produced no row: either the
// workspace already holds this name, the free-tier quota is exhausted, or a
// concurrent creation won the race.
func (s *Store) classifyCreateRejection(ctx context.Context, orgID, name string) error {
	var exists bool
	existsErr := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM applications
		 WHERE org_id = $1 AND name = $2 AND deleted_at IS NULL)`,
		orgID, name).Scan(&exists)
	if existsErr != nil {
		return existsErr
	}
	if exists {
		return fmt.Errorf("%w: name already used in this workspace", ErrConflict)
	}
	var count int
	countErr := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM applications WHERE org_id = $1 AND deleted_at IS NULL`,
		orgID).Scan(&count)
	if countErr != nil {
		return countErr
	}
	if count >= freeTierMaxApps {
		return fmt.Errorf("%w: free tier allows %d apps", ErrQuotaExceeded, freeTierMaxApps)
	}
	return fmt.Errorf("%w: concurrent creation race", ErrConflict)
}

func (s *Store) GetApp(ctx context.Context, orgID, appID string) (App, error) {
	if !validUUID(appID) {
		return App{}, ErrNotFound
	}
	var app App
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at::text FROM applications
		 WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		appID, orgID).Scan(&app.ID, &app.Name, &app.Slug, &app.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return App{}, ErrNotFound
	}
	return app, err
}

func (s *Store) ListApps(ctx context.Context, orgID string) ([]App, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, slug, created_at::text FROM applications
		 WHERE org_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []App{}
	for rows.Next() {
		var a App
		if scanErr := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) RenameApp(ctx context.Context, orgID, appID, name string) (App, error) {
	if !validUUID(appID) {
		return App{}, ErrNotFound
	}
	slug := slugify(name)
	var app App
	updateErr := s.pool.QueryRow(ctx,
		`UPDATE applications SET name = $3, slug = $4, updated_at = now()
		 WHERE id = $2 AND org_id = $1 AND deleted_at IS NULL
		 RETURNING id, name, slug, created_at::text`,
		orgID, appID, name, slug).
		Scan(&app.ID, &app.Name, &app.Slug, &app.CreatedAt)
	if isUniqueViolation(updateErr) {
		return App{}, fmt.Errorf("%w: name already used in this workspace", ErrConflict)
	}
	if errors.Is(updateErr, pgx.ErrNoRows) {
		return App{}, ErrNotFound
	}
	return app, updateErr
}

func (s *Store) SoftDeleteApp(ctx context.Context, orgID, appID string) error {
	if !validUUID(appID) {
		return ErrNotFound
	}
	tag, execErr := s.pool.Exec(ctx,
		`UPDATE applications SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		appID, orgID)
	if execErr != nil {
		return execErr
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var envNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,28}$`)

type ServiceRow struct {
	ID      string
	AppID   string
	Name    string
	Type    string
	Runtime string
	Port    int
}

func (s *Store) AddService(
	ctx context.Context, orgID, appID, name, svcType, runtime string, port int,
) (ServiceRow, error) {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return ServiceRow{}, getAppErr
	}
	var svc ServiceRow
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO services (app_id, name, type, runtime, port)
		 VALUES ($1, $2, $3, $4, NULLIF($5, 0))
		 ON CONFLICT (app_id, name) DO NOTHING
		 RETURNING id, app_id::text, name, type, runtime, COALESCE(port, 0)`,
		appID, name, svcType, runtime, port).
		Scan(&svc.ID, &svc.AppID, &svc.Name, &svc.Type, &svc.Runtime, &svc.Port)
	if errors.Is(insertErr, pgx.ErrNoRows) {
		return ServiceRow{}, fmt.Errorf("%w: service name exists", ErrConflict)
	}
	if isUniqueViolation(insertErr) {
		return ServiceRow{}, fmt.Errorf("%w: service name exists", ErrConflict)
	}
	return svc, insertErr
}

// AddEnvironment provisions the environment row including its deterministic
// future namespace label app-{shortAppID}-{env} (RB-5 naming rule).
func (s *Store) AddEnvironment(ctx context.Context, orgID, appID, envName string) (string, error) {
	if !envNameRE.MatchString(envName) {
		return "", fmt.Errorf("%w: environment name must be lowercase alphanumeric", ErrConflict)
	}
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return "", getAppErr
	}
	shortID := strings.SplitN(appID, "-", 2)[0]
	namespace := fmt.Sprintf("app-%s-%s", shortID, envName)
	var id string
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO environments (app_id, name, namespace_name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, name) DO NOTHING
		 RETURNING id`,
		appID, envName, namespace).Scan(&id)
	if errors.Is(insertErr, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: environment exists", ErrConflict)
	}
	if isUniqueViolation(insertErr) {
		return "", fmt.Errorf("%w: environment exists", ErrConflict)
	}
	return namespace, insertErr
}
