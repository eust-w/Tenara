package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"

	"github.com/jackc/pgx/v5"

	"tenara/control-plane/internal/rbac"
)

type roleKey struct{}

// RoleFromContext exposes the verified role to downstream handlers.
func RoleFromContext(r *http.Request) (rbac.Role, bool) {
	role, ok := r.Context().Value(roleKey{}).(rbac.Role)
	return role, ok
}

// requireCap is the single authorization gate: every capability-protected
// route must be wrapped here; scattered role checks are forbidden (RB-6).
func (s *Service) requireCap(needed rbac.Capability, next http.HandlerFunc) http.HandlerFunc {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request) {
		ident, identOK := identityFrom(r)
		if !identOK {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
			return
		}
		role, roleErr := s.store.RoleInOrg(r.Context(), ident.UserID, ident.OrgID)
		if roleErr != nil || !rbac.Can(role, needed) {
			writeProblem(w, http.StatusForbidden, "FORBIDDEN_CAPABILITY", "missing "+string(needed))
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), roleKey{}, role)))
	})
}

var errUserNotFound = errors.New("user not found")

type MemberRow struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func (s *Store) RoleInOrg(ctx context.Context, userID, orgID string) (rbac.Role, error) {
	var role rbac.Role
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM organization_members WHERE user_id = $1 AND org_id = $2`,
		userID, orgID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errUserNotFound
	}
	return role, err
}

func (s *Store) ListMembers(ctx context.Context, orgID string) ([]MemberRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT m.user_id, u.email::text, m.role
		 FROM organization_members m JOIN users u ON u.id = m.user_id
		 WHERE m.org_id = $1 ORDER BY u.email`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemberRow{}
	for rows.Next() {
		var m MemberRow
		if scanErr := rows.Scan(&m.UserID, &m.Email, &m.Role); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InviteMemberByEmail adds (or re-roles) a membership keyed by email. When no
// account exists yet, an unverified user stub is provisioned so the invitee
// can claim it later via the password-reset flow.
func (s *Store) InviteMemberByEmail(
	ctx context.Context, orgID, email string, role rbac.Role,
) (userID string, created bool, err error) {
	tx, txErr := s.pool.Begin(ctx)
	if txErr != nil {
		return "", false, txErr
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil && !errors.Is(rollErr, pgx.ErrTxClosed) {
			return
		}
	}()

	selectErr := tx.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	switch {
	case errors.Is(selectErr, pgx.ErrNoRows):
		pwdSource, pwdErr := RandomToken()
		if pwdErr != nil {
			return "", false, pwdErr
		}
		hash, hashErr := HashPassword(pwdSource)
		if hashErr != nil {
			return "", false, hashErr
		}
		insertErr := tx.QueryRow(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
			email, hash).Scan(&userID)
		if insertErr != nil {
			return "", false, insertErr
		}
		created = true
	case selectErr != nil:
		return "", false, selectErr
	}

	if _, execErr := tx.Exec(ctx,
		`INSERT INTO organization_members (user_id, org_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, org_id) DO UPDATE SET role = EXCLUDED.role`,
		userID, orgID, role); execErr != nil {
		return "", false, execErr
	}
	return userID, created, tx.Commit(ctx)
}

type inviteInput struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (s *Service) handleListMembers(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	members, listErr := s.store.ListMembers(r.Context(), ident.OrgID)
	if listErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "list failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(members)
}

func (s *Service) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	var in inviteInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	role := rbac.Role(in.Role)
	if _, addrErr := mail.ParseAddress(in.Email); addrErr != nil || !rbac.IsValidRole(role) {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid email or role")
		return
	}
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	userID, created, inviteErr := s.store.InviteMemberByEmail(
		r.Context(), ident.OrgID, in.Email, role)
	if inviteErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "invite failed")
		return
	}
	status := "existed"
	if created {
		status = "provisioned"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"user_id": userID, "email": in.Email, "role": string(role), "status": status,
	})
}

func (s *Store) ensureBootstrapUser(ctx context.Context, adminEmail string, userID *string) error {
	pwdSource, pwdErr := RandomToken()
	if pwdErr != nil {
		return pwdErr
	}
	hash, hashErr := HashPassword(pwdSource)
	if hashErr != nil {
		return hashErr
	}
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		adminEmail, hash).Scan(userID)
	if insertErr != nil {
		return fmt.Errorf("bootstrap admin insert: %w", insertErr)
	}
	return nil
}

// EnsurePlatformAdmin backs ADMIN_EMAIL first-boot bootstrap (RB-6): the user
// (created unverified if absent) receives platform_admin in their default org.
func (s *Store) EnsurePlatformAdmin(ctx context.Context, adminEmail string) error {
	var userID string
	selectErr := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1`, adminEmail).Scan(&userID)
	switch {
	case errors.Is(selectErr, pgx.ErrNoRows):
		if ensureErr := s.ensureBootstrapUser(ctx, adminEmail, &userID); ensureErr != nil {
			return ensureErr
		}
	case selectErr != nil:
		return selectErr
	}
	orgID, orgErr := s.DefaultOrgForUser(ctx, userID)
	if orgErr != nil {
		return orgErr
	}
	_, execErr := s.pool.Exec(ctx,
		`INSERT INTO organization_members (user_id, org_id, role) VALUES ($1, $2, 'platform_admin')
		 ON CONFLICT (user_id, org_id) DO UPDATE SET role = 'platform_admin'`,
		userID, orgID)
	return execErr
}
