package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"net/smtp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotVerified        = errors.New("email not verified")
	ErrWeakPassword       = errors.New("weak password")
	ErrRateLimited        = errors.New("too many signups from this IP")
	ErrBadToken           = errors.New("token invalid or expired")
)

type User struct {
	ID            string
	Email         string
	PasswordHash  string
	EmailVerified bool
}

// MailSender delivers platform emails (MailHog in dev).
type MailSender interface {
	SendVerification(to mail.Address, verifyURL string) error
}

type SMTPSender struct {
	Addr string // host:port
}

func (s SMTPSender) SendVerification(to mail.Address, verifyURL string) error {
	body := fmt.Sprintf("To: %s\r\nSubject: Verify your Tenara account\r\n\r\nVerify: %s\r\n", to.Address, verifyURL)
	return smtp.SendMail(s.Addr, nil, "noreply@tenara.local", []string{to.Address}, []byte(body))
}

const (
	signupsPerWindow   = 5
	signupWindowLength = time.Hour
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) CreateUnverified(ctx context.Context, email, passwordHash string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 ON CONFLICT (email) DO NOTHING
		 RETURNING id, email, password_hash, email_verified`,
		email, passwordHash)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (s *Store) GetByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, email_verified FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	return u, err
}

func (s *Store) StoreVerification(ctx context.Context, userID string, tokenHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO email_verifications (token_hash, user_id) VALUES ($1, $2)`, tokenHash, userID)
	return err
}

func (s *Store) ConsumeVerification(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx,
		`UPDATE users SET email_verified = true, updated_at = now()
		 WHERE id = (SELECT user_id FROM email_verifications
		             WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now())
		 RETURNING id`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrBadToken
	}
	if err != nil {
		return "", err
	}
	_, execErr := s.pool.Exec(ctx,
		`UPDATE email_verifications SET consumed_at = now() WHERE token_hash = $1`, tokenHash)
	return userID, execErr
}

func (s *Store) StoreResetToken(ctx context.Context, userID string, tokenHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO password_resets (token_hash, user_id) VALUES ($1, $2)`, tokenHash, userID)
	return err
}

// ConsumeResetToken is single-use and updates the password atomically.
func (s *Store) ConsumeResetToken(ctx context.Context, tokenHash, newPasswordHash string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now()
		 WHERE id = (SELECT user_id FROM password_resets
		             WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now())`,
		tokenHash, newPasswordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBadToken
	}
	_, execErr := s.pool.Exec(ctx,
		`UPDATE password_resets SET consumed_at = now() WHERE token_hash = $1`, tokenHash)
	return execErr
}

// CheckSignupLimit enforces N signups per IP per rolling hour window.
func (s *Store) CheckSignupLimit(ctx context.Context, ip string) error {
	addr, parseErr := netip.ParseAddr(ip)
	if parseErr != nil {
		return nil // unparseable IP context; do not block
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil && !errors.Is(rollErr, pgx.ErrTxClosed) {
			return
		}
	}()

	var windowStart time.Time
	var count int
	err = tx.QueryRow(ctx,
		`SELECT window_start, count FROM signup_rate_limits WHERE ip = $1 FOR UPDATE`, addr).
		Scan(&windowStart, &count)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, insertErr := tx.Exec(ctx,
			`INSERT INTO signup_rate_limits (ip, count) VALUES ($1, 1)`, addr)
		if insertErr != nil {
			return insertErr
		}
	case err != nil:
		return err
	case time.Since(windowStart) > signupWindowLength:
		_, updateErr := tx.Exec(ctx,
			`UPDATE signup_rate_limits SET count = 1, window_start = now() WHERE ip = $1`, addr)
		if updateErr != nil {
			return updateErr
		}
	default:
		if count >= signupsPerWindow {
			return ErrRateLimited
		}
		_, updateErr := tx.Exec(ctx,
			`UPDATE signup_rate_limits SET count = count + 1 WHERE ip = $1`, addr)
		if updateErr != nil {
			return updateErr
		}
	}
	return tx.Commit(ctx)
}
