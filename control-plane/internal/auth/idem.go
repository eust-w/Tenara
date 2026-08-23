package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type captureRW struct {
	http.ResponseWriter
	buf         bytes.Buffer
	status      int
	wroteHeader bool
}

func (c *captureRW) WriteHeader(code int) {
	if !c.wroteHeader {
		c.status = code
		c.wroteHeader = true
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureRW) Write(b []byte) (int, error) {
	//nolint:errcheck,revive // mirror capture is best-effort telemetry
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

func mutatingRequest(r *http.Request) bool {
	return r.Method != http.MethodGet && r.Method != http.MethodHead &&
		r.Method != http.MethodOptions
}

// idem enforces RB-33/R2 semantics on mutations carrying Idempotency-Key:
// replays return the stored response plus Idempotent-Replayed:true, the same
// key with a different payload is rejected 422. GET never hits this path.
func (s *Service) idem(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mutatingRequest(r) {
			next(w, r)
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r)
			return
		}
		ident, ok := identityFrom(r)
		if !ok {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
			return
		}
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			writeProblem(w, http.StatusBadRequest, "VALIDATION_FAILED", "unreadable body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		sum := sha256.Sum256([]byte(r.Method + r.URL.Path + string(raw)))
		hash := hex.EncodeToString(sum[:])

		first, claimErr := s.store.ClaimIdempotency(r.Context(), key, ident.OrgID, hash)
		if claimErr != nil {
			writeProblem(w, http.StatusInternalServerError, "INTERNAL", "idempotency store failed")
			return
		}
		if !first {
			status, body, loadErr := s.store.LoadIdempotentResponse(r.Context(), key, ident.OrgID, hash)
			if loadErr != nil {
				writeProblem(w, loadErr.status, loadErr.code, loadErr.detail)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(status)
			//nolint:errcheck // replay to caller; nothing actionable on failure
			_, _ = w.Write(body)
			return
		}

		cw := &captureRW{ResponseWriter: w, status: http.StatusOK}
		next(cw, r)
		if storeErr := s.store.CompleteIdempotency(
			r.Context(), key, ident.OrgID, cw.status, cw.buf.Bytes()); storeErr != nil {
			// The caller already got its response; nothing else to do here.
			_ = storeErr
		}
	}
}

type idemHTTPError struct {
	code   string
	detail string
	status int
}

// ClaimIdempotency atomically claims a fresh key (purging any expired row
// first); returning false means the key already exists and was not expired.
func (s *Store) ClaimIdempotency(ctx context.Context, key, orgID, requestHash string) (bool, error) {
	if _, execErr := s.pool.Exec(ctx,
		`DELETE FROM idempotency_keys
		 WHERE idempotency_key = $1 AND org_id = $2 AND expires_at <= now()`,
		key, orgID); execErr != nil {
		return false, execErr
	}
	var gotHash string
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO idempotency_keys (idempotency_key, org_id, request_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (idempotency_key, org_id) DO NOTHING
		 RETURNING request_hash`, key, orgID, requestHash).Scan(&gotHash)
	if errors.Is(insertErr, pgx.ErrNoRows) {
		return false, nil
	}
	if insertErr != nil {
		return false, insertErr
	}
	return true, nil
}

// LoadIdempotentResponse returns the stored outcome for a claimed key,
// translated into HTTP problem details for mismatched or still-running calls.
func (s *Store) LoadIdempotentResponse(
	ctx context.Context, key, orgID, requestHash string,
) (int, []byte, *idemHTTPError) {
	var (
		storedHash string
		status     *int
		body       []byte
	)
	scanErr := s.pool.QueryRow(ctx,
		`SELECT request_hash, response_status, response_body::text
		 FROM idempotency_keys WHERE idempotency_key = $1 AND org_id = $2`,
		key, orgID).Scan(&storedHash, &status, &body)
	if scanErr != nil {
		return 0, nil, &idemHTTPError{
			code: "INTERNAL", detail: "idempotency lookup failed", status: http.StatusInternalServerError,
		}
	}
	if storedHash != requestHash {
		return 0, nil, &idemHTTPError{
			code: "IDEMPOTENCY_CONFLICT", status: http.StatusUnprocessableEntity,
			detail: "key already used with a different payload",
		}
	}
	if status == nil {
		return 0, nil, &idemHTTPError{
			code: "IDEMPOTENCY_IN_FLIGHT", status: http.StatusConflict,
			detail: "original request still running",
		}
	}
	return *status, body, nil
}

func (s *Store) CompleteIdempotency(
	ctx context.Context, key, orgID string, status int, body []byte,
) error {
	if len(body) == 0 {
		body = []byte("{}")
	}
	_, execErr := s.pool.Exec(ctx,
		`UPDATE idempotency_keys SET response_status = $3, response_body = $4
		 WHERE idempotency_key = $1 AND org_id = $2`,
		key, orgID, status, string(body))
	return execErr
}

// CleanupExpiredIdempotency purges terminal-state rows past their 24h TTL.
func (s *Store) CleanupExpiredIdempotency(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM idempotency_keys WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// StartIdempotencyCleanup runs the hourly purge until ctx is cancelled.
func StartIdempotencyCleanup(ctx context.Context, store *Store) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				//nolint:errcheck // best-effort purge; retried on next tick
				_, _ = store.CleanupExpiredIdempotency(ctx)
			}
		}
	}()
}
