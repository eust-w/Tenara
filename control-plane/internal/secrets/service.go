// Package secrets implements the encrypted secret store behind the KMS stub
// (plan tenara-agent-paas#19, RB-22 R11). Plaintext never leaves SetSecret;
// reads surface only the masked sentinel value unless explicitly revealed.
package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/kms"
)

var ErrNotFound = errors.New("secret not found")

// Item is the masked listing entry: values are never returned here.
type Item struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Service struct {
	pool *pgxpool.Pool
	kms  *kms.Stub
}

func New(pool *pgxpool.Pool, kmsStub *kms.Stub) *Service {
	return &Service{pool: pool, kms: kmsStub}
}

// SetSecret encrypts the plaintext with the workspace master key and stores
// both the live row and an immutable revision (version = max+1).
func (s *Service) SetSecret(ctx context.Context, appID, name, plaintext string) error {
	sealed, encErr := s.kms.Encrypt([]byte(plaintext))
	if encErr != nil {
		return fmt.Errorf("encrypt: %w", encErr)
	}
	tx, txErr := s.pool.Begin(ctx)
	if txErr != nil {
		return txErr
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil && !errors.Is(rollErr, pgx.ErrTxClosed) {
			return
		}
	}()

	var secretID string
	var nextVersion int
	upsertErr := tx.QueryRow(ctx,
		`INSERT INTO secrets (app_id, name, ciphertext)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, name) DO UPDATE
		     SET ciphertext = EXCLUDED.ciphertext, updated_at = now()
		 RETURNING id,
		           COALESCE((SELECT max(version) FROM secret_revisions
		             WHERE secret_id = secrets.id), 0) + 1`,
		appID, name, sealed).Scan(&secretID, &nextVersion)
	if upsertErr != nil {
		return upsertErr
	}
	if _, revErr := tx.Exec(ctx,
		`INSERT INTO secret_revisions (secret_id, version, ciphertext)
		 VALUES ($1, $2, $3)`,
		secretID, nextVersion, sealed); revErr != nil {
		return revErr
	}
	return tx.Commit(ctx)
}

// ListSecrets returns names only; every value is the masked sentinel.
func (s *Service) ListSecrets(ctx context.Context, appID string) ([]Item, error) {
	rows, queryErr := s.pool.Query(ctx,
		`SELECT name FROM secrets WHERE app_id = $1 ORDER BY name`, appID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, Item{Name: name, Value: "configured"})
	}
	return out, rows.Err()
}

// Reveal decrypts and returns the plaintext for one secret.
func (s *Service) Reveal(ctx context.Context, appID, name string) (string, error) {
	var sealed []byte
	scanErr := s.pool.QueryRow(ctx,
		`SELECT ciphertext FROM secrets WHERE app_id = $1 AND name = $2`,
		appID, name).Scan(&sealed)
	if errors.Is(scanErr, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if scanErr != nil {
		return "", scanErr
	}
	plain, decErr := s.kms.Decrypt(sealed)
	if decErr != nil {
		return "", fmt.Errorf("decrypt: %w", decErr)
	}
	return string(plain), nil
}
