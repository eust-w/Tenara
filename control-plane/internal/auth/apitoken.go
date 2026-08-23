package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	apiTokenPrefix   = "tenara_"
	apiTokenDisplayN = 6 // chars of secret shown after the fixed prefix
)

var ErrTokenNotFound = errors.New("token not found")

// GenerateAPIToken produces a one-time plaintext, its display prefix and the
// at-rest sha256 hash. Only the hash is persisted.
func GenerateAPIToken() (plaintext, displayPrefix, hash string, err error) {
	raw := make([]byte, 24)
	if _, readErr := rand.Read(raw); readErr != nil {
		return "", "", "", fmt.Errorf("read token entropy: %w", readErr)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	plaintext = apiTokenPrefix + secret
	displayPrefix = apiTokenPrefix + secret[:apiTokenDisplayN]
	sum := sha256.Sum256([]byte(plaintext))
	return plaintext, displayPrefix, hex.EncodeToString(sum[:]), nil
}

func HashAPIToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// DefaultOrgForUser returns the user's first membership, creating a personal
// workspace on first use so every user always has exactly one default org.
func (s *Store) DefaultOrgForUser(ctx context.Context, userID string) (string, error) {
	var orgID string
	scanErr := s.pool.QueryRow(ctx,
		`SELECT org_id FROM organization_members WHERE user_id = $1 ORDER BY role LIMIT 1`,
		userID).Scan(&orgID)
	if scanErr == nil {
		return orgID, nil
	}
	if !errors.Is(scanErr, pgx.ErrNoRows) {
		return "", scanErr
	}

	var email string
	if emailErr := s.pool.QueryRow(ctx,
		`SELECT email FROM users WHERE id = $1`, userID).Scan(&email); emailErr != nil {
		return "", emailErr
	}
	local := email
	if at := indexByte(local, '@'); at > 0 {
		local = local[:at]
	}
	tx, txErr := s.pool.Begin(ctx)
	if txErr != nil {
		return "", txErr
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil && !errors.Is(rollErr, pgx.ErrTxClosed) {
			return
		}
	}()

	sfx, sfxErr := RandomToken()
	if sfxErr != nil {
		return "", sfxErr
	}
	orgErr := tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id`,
		local+"'s workspace", "ws-"+userID[:8]+"-"+sfx[:6]).
		Scan(&orgID)
	if orgErr != nil {
		return "", orgErr
	}
	if _, execErr := tx.Exec(ctx,
		`INSERT INTO organization_members (user_id, org_id, role) VALUES ($1, $2, 'workspace_admin')`,
		userID, orgID); execErr != nil {
		return "", execErr
	}
	return orgID, tx.Commit(ctx)
}

func indexByte(s string, b byte) int {
	for i := range len(s) {
		if s[i] == b {
			return i
		}
	}
	return -1
}

type APITokenRow struct {
	ID        string
	Name      string
	Prefix    string
	CreatedAt string
}

func (s *Store) CreateAPIToken(ctx context.Context, userID, orgID, name, hash, prefix string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO api_tokens (org_id, created_by, name, token_hash, prefix)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		orgID, userID, name, hash, prefix).Scan(&id)
	return id, err
}

// ResolveAPIToken maps a bearer plaintext to its live user+org scope.
func (s *Store) ResolveAPIToken(ctx context.Context, plaintext string) (userID, orgID string, err error) {
	hash := HashAPIToken(plaintext)
	err = s.pool.QueryRow(ctx,
		`SELECT t.created_by, t.org_id FROM api_tokens t
		 WHERE t.token_hash = $1 AND t.revoked_at IS NULL`, hash).
		Scan(&userID, &orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrTokenNotFound
	}
	if err != nil {
		return "", "", err
	}
	//nolint:errcheck // last-used tracking is best-effort telemetry
	_, _ = s.pool.Exec(ctx,
		`UPDATE api_tokens SET last_used_at = now() WHERE token_hash = $1`, hash)
	return userID, orgID, nil
}

func (s *Store) ListAPITokens(ctx context.Context, userID string) ([]APITokenRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, prefix, created_at::text FROM api_tokens
		 WHERE created_by = $1 AND revoked_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APITokenRow
	for rows.Next() {
		var r APITokenRow
		if scanErr := rows.Scan(&r.ID, &r.Name, &r.Prefix, &r.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RevokeAPIToken(ctx context.Context, userID, tokenID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE api_tokens SET revoked_at = now()
		 WHERE id = $1 AND created_by = $2 AND revoked_at IS NULL`, tokenID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
