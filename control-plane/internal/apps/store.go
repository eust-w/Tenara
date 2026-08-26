// Package apps implements application/service/environment management
// (plan tenara-agent-paas#16, RB-5 RB-29 R10). Storage only: no Kubernetes
// resources are touched here, rendering belongs to the controllers.
package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// freeTierMaxApps pins the R10 org-level quota; namespace-level ResourceQuota
// is layered separately by the tenant controller (todo37).
const (
	// freeTierMaxApps pins the R10 org-level app quota for the free tier.
	freeTierMaxApps = 3
	// proTierMaxApps lifts the ceiling for pro organizations.
	proTierMaxApps = 50
)

// OrgTier returns the organization billing tier ("free" when unset).
func (s *Store) OrgTier(ctx context.Context, orgID string) (string, error) {
	var tier string
	err := s.pool.QueryRow(ctx,
		`SELECT tier FROM organizations WHERE id = $1`, orgID).Scan(&tier)
	if errors.Is(err, pgx.ErrNoRows) {
		return "free", nil
	}
	return tier, err
}

func tierOrFree(tier string) string {
	if tier == "" {
		return "free"
	}
	return tier
}

var (
	ErrNotFound      = errors.New("not found")
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrUnsupportedStackKind mirrors analyzer.ErrUnsupportedStack across the
	// module boundary without importing the analyzer package.
	ErrUnsupportedStackKind = errors.New("unsupported stack")
	ErrConflict             = errors.New("conflict")
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
	var tier string
	tierErr := s.pool.QueryRow(ctx,
		`SELECT tier FROM organizations WHERE id = $1`, orgID).Scan(&tier)
	if errors.Is(tierErr, pgx.ErrNoRows) {
		return App{}, ErrNotFound
	}
	if tierErr != nil {
		return App{}, tierErr
	}
	maxApps := freeTierMaxApps
	if tier == "pro" {
		maxApps = proTierMaxApps
	}

	var app App
	insertErr := s.pool.QueryRow(ctx, `
INSERT INTO applications (org_id, name, slug, created_by)
SELECT $1, $2, $3, NULLIF($4,'')::uuid
WHERE (SELECT count(*) FROM applications WHERE org_id = $1 AND deleted_at IS NULL) < $5
ON CONFLICT (org_id, name) DO NOTHING
RETURNING id, name, slug, created_at::text`,
		orgID, name, slug, createdBy, maxApps).
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
	// Tier-aware cap (RB§29): organizations.tier decides the ceiling.
	var tier string
	tierErr := s.pool.QueryRow(ctx,
		`SELECT tier FROM organizations WHERE id = $1`, orgID).Scan(&tier)
	if tierErr != nil && !errors.Is(tierErr, pgx.ErrNoRows) {
		return tierErr
	}
	maxApps := freeTierMaxApps
	if tier == "pro" {
		maxApps = proTierMaxApps
	}
	if count >= maxApps {
		return fmt.Errorf("%w: %s tier allows %d apps",
			ErrQuotaExceeded, tierOrFree(tier), maxApps)
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

// UpdateSpec persists the manual AppSpec override (R8) for an app the caller
// owns; soft-deleted or foreign apps surface as ErrNotFound.
func (s *Store) UpdateSpec(ctx context.Context, orgID, appID string, raw []byte) error {
	if !validUUID(appID) {
		return ErrNotFound
	}
	tag, execErr := s.pool.Exec(ctx,
		`UPDATE applications SET current_spec = $3::jsonb, updated_at = now()
		 WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		appID, orgID, string(raw))
	if execErr != nil {
		return execErr
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrPlanExpired marks an approval attempt on a past-TTL plan.
var ErrPlanExpired = errors.New("plan expired")

// EnsureEnvironment returns the environment row id for (app, env),
// creating it on first use.
func (s *Store) EnsureEnvironment(ctx context.Context, orgID, appID, envName string) (string, error) {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return "", getAppErr
	}
	if !envNameRE.MatchString(envName) {
		return "", fmt.Errorf("%w: invalid environment name", ErrConflict)
	}
	shortID := strings.SplitN(appID, "-", 2)[0]
	namespace := fmt.Sprintf("app-%s-%s", shortID, envName)
	var envID string
	upsertErr := s.pool.QueryRow(ctx,
		`INSERT INTO environments (app_id, name, namespace_name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		appID, envName, namespace).Scan(&envID)
	return envID, upsertErr
}

// SaveAwaitingPlan persists a deployment row in AWAITING_APPROVAL carrying
// the rendered plan snapshot; the generated plan_id doubles as public handle.
func (s *Store) SaveAwaitingPlan(
	ctx context.Context, orgID, appID, envName string, planSnapshot []byte,
) (deploymentID, planID string, err error) {
	envID, ensureErr := s.EnsureEnvironment(ctx, orgID, appID, envName)
	if ensureErr != nil {
		return "", "", ensureErr
	}
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO deployments (app_id, environment_id, state, plan_id, plan_snapshot)
		 VALUES ($1, $2, 'AWAITING_APPROVAL', gen_random_uuid(), $3)
		 RETURNING id::text, plan_id::text`,
		appID, envID, string(planSnapshot)).Scan(&deploymentID, &planID)
	return deploymentID, planID, insertErr
}

type AppWithSpec struct {
	App
	SpecRaw []byte
}

// SaveAppSpec persists a validated AppSpec document onto the application.
func (s *Store) SaveAppSpec(ctx context.Context, orgID, appID string, specRaw []byte) error {
	if !validUUID(appID) {
		return ErrNotFound
	}
	ct, execErr := s.pool.Exec(ctx,
		`UPDATE applications SET current_spec = $2
		 WHERE id = $1 AND org_id = $3 AND deleted_at IS NULL`,
		appID, specRaw, orgID)
	if execErr != nil {
		return execErr
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAppWithSpec additionally surfaces the stored manual override.
func (s *Store) GetAppWithSpec(ctx context.Context, orgID, appID string) (AppWithSpec, error) {
	if !validUUID(appID) {
		return AppWithSpec{}, ErrNotFound
	}
	var out AppWithSpec
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at::text, COALESCE(current_spec::text,'')
		 FROM applications WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		appID, orgID).
		Scan(&out.ID, &out.Name, &out.Slug, &out.CreatedAt, &out.SpecRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return AppWithSpec{}, ErrNotFound
	}
	return out, err
}

// ApprovePlan validates the caller-owned awaiting plan (expiry + state) and
// transitions it to PLANNED.
func (s *Store) ApprovePlan(ctx context.Context, orgID, appID, planID string) (string, error) {
	if !validUUID(planID) {
		return "", ErrNotFound
	}
	var (
		depID    string
		state    string
		snapshot []byte
	)
	scanErr := s.pool.QueryRow(ctx,
		`SELECT d.id::text, d.state, d.plan_snapshot::text
		 FROM deployments d JOIN applications a ON a.id = d.app_id
		 WHERE d.plan_id = $1 AND a.org_id = $2 AND a.id = $3`,
		planID, orgID, appID).Scan(&depID, &state, &snapshot)
	if errors.Is(scanErr, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if scanErr != nil {
		return "", scanErr
	}
	var snap struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if parseErr := json.Unmarshal(snapshot, &snap); parseErr != nil {
		return "", fmt.Errorf("%w: corrupt snapshot", ErrConflict)
	}
	if time.Now().After(snap.ExpiresAt) {
		return "", ErrPlanExpired
	}
	if state != "AWAITING_APPROVAL" {
		return "", fmt.Errorf("%w: plan already %s", ErrConflict, state)
	}
	if _, execErr := s.pool.Exec(ctx,
		`UPDATE deployments SET state = 'PLANNED', updated_at = now()
		 WHERE id = $1`,
		depID); execErr != nil {
		return "", execErr
	}
	return depID, nil
}

// AuditRow is a generic audit_logs insert used by custom-result flows
// (e.g. secret.revealed) outside the standard audited middleware.
type AuditRow struct {
	ActorType   string
	ActorID     string
	Agent       string
	WorkspaceID string
	Action      string
	SourceIP    string
	RequestID   string
	Result      string
}

func (s *Store) InsertAuditRow(ctx context.Context, e AuditRow) error {
	_, execErr := s.pool.Exec(ctx,
		`INSERT INTO audit_logs
		 (actor_type, actor_id, agent, workspace_id, action, source_ip, request_id, result)
		 VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,''), NULLIF($4,'')::uuid,
		         $5, NULLIF($6,'')::inet, NULLIF($7,''), $8)`,
		e.ActorType, e.ActorID, e.Agent, e.WorkspaceID,
		e.Action, e.SourceIP, e.RequestID, e.Result)
	return execErr
}

// RollbackLatest restores the previous revision of the app's newest
// deployment as a fresh revision (RB-26 R1).
func (s *Store) RollbackLatest(
	ctx context.Context, orgID, appID string,
) (RevisionRow, RevisionRow, error) {
	if !validUUID(appID) {
		return RevisionRow{}, RevisionRow{}, ErrNotFound
	}
	var depID string
	depErr := s.pool.QueryRow(ctx,
		`SELECT dp.id FROM deployments dp
		 JOIN applications ap ON ap.id = dp.app_id
		 WHERE ap.org_id = $1 AND ap.id = $2 AND ap.deleted_at IS NULL
		 ORDER BY dp.id DESC LIMIT 1`, orgID, appID).Scan(&depID)
	if errors.Is(depErr, pgx.ErrNoRows) {
		return RevisionRow{}, RevisionRow{}, fmt.Errorf("%w: no deployment yet", ErrNotEnoughRevisions)
	}
	if depErr != nil {
		return RevisionRow{}, RevisionRow{}, depErr
	}
	target, tgtErr := s.RollbackTarget(ctx, orgID, appID, depID)
	if tgtErr != nil {
		return RevisionRow{}, RevisionRow{}, tgtErr
	}
	newRev, saveErr := s.SaveRevision(ctx, orgID, appID, depID, RevisionInput{
		GitSHA:         target.GitSHA,
		BuildID:        target.BuildID,
		ImageDigest:    target.ImageDigest,
		ConfigVersion:  target.ConfigVersion,
		SecretRevision: target.SecretRevision,
		AppSpecVersion: target.AppSpecVersion,
	})
	return newRev, target, saveErr
}
