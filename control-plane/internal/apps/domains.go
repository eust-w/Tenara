package apps

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// DNSResolver abstracts TXT lookups so ownership verification can be mocked
// in tests and swapped for other providers later.
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type DomainRow struct {
	ID           string `json:"id"`
	AppID        string `json:"app_id,omitempty"`
	Hostname     string `json:"hostname"`
	Verified     bool   `json:"verified"`
	TXTChallenge string `json:"txt_challenge,omitempty"`
	CNameTarget  string `json:"cname_target,omitempty"`
	IsDefault    bool   `json:"is_default"`
}

const challengePrefix = "_tenara-challenge."

func newChallengeToken() (string, error) {
	raw := make([]byte, 24)
	if _, readErr := rand.Read(raw); readErr != nil {
		return "", fmt.Errorf("read challenge entropy: %w", readErr)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Store) getDomain(ctx context.Context, appID, domainID string) (DomainRow, error) {
	var d DomainRow
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, app_id::text, hostname::text, verified,
		        COALESCE(txt_challenge,''), COALESCE(cname_target,''), is_default
		 FROM domains WHERE id = $1 AND app_id = $2`,
		domainID, appID).Scan(&d.ID, &d.AppID, &d.Hostname, &d.Verified,
		&d.TXTChallenge, &d.CNameTarget, &d.IsDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainRow{}, ErrNotFound
	}
	return d, err
}

func (s *Store) insertDomain(ctx context.Context, d DomainRow) (DomainRow, error) {
	insertErr := s.pool.QueryRow(ctx,
		`INSERT INTO domains (app_id, hostname, verified, txt_challenge, is_default)
		 VALUES ($1, $2, $3, NULLIF($4,''), $5)
		 RETURNING id::text`,
		d.AppID, strings.ToLower(d.Hostname), d.Verified,
		d.TXTChallenge, d.IsDefault).Scan(&d.ID)
	if isUniqueViolation(insertErr) {
		return DomainRow{}, fmt.Errorf("%w: hostname already bound", ErrConflict)
	}
	return d, insertErr
}

// AllocateDefaultDomain creates <slug>.<baseDomain> instantly verified (RB-25):
// the subdomain lives under our own zone, so no ownership challenge applies.
func (s *Store) AllocateDefaultDomain(
	ctx context.Context, appID, slug, baseDomain string,
) (DomainRow, error) {
	return s.insertDomain(ctx, DomainRow{
		AppID:     appID,
		Hostname:  fmt.Sprintf("%s.%s", slug, baseDomain),
		Verified:  true,
		IsDefault: true,
	})
}

// AddCustomDomain stages an unverified custom hostname plus its TXT challenge.
func (s *Store) AddCustomDomain(ctx context.Context, appID, hostname string) (DomainRow, error) {
	token, tokenErr := newChallengeToken()
	if tokenErr != nil {
		return DomainRow{}, tokenErr
	}
	return s.insertDomain(ctx, DomainRow{
		AppID:        appID,
		Hostname:     hostname,
		Verified:     false,
		TXTChallenge: token,
	})
}

func (s *Store) markDomainVerified(ctx context.Context, domainID string) error {
	tag, execErr := s.pool.Exec(ctx,
		`UPDATE domains SET verified = true WHERE id = $1 AND verified = false`, domainID)
	if execErr != nil {
		return execErr
	}
	_ = tag
	return nil
}

func (s *Store) listDomains(ctx context.Context, appID string) ([]DomainRow, error) {
	rows, queryErr := s.pool.Query(ctx,
		`SELECT id::text, app_id::text, hostname::text, verified,
		        COALESCE(txt_challenge,''), COALESCE(cname_target,''), is_default
		 FROM domains WHERE app_id = $1 ORDER BY is_default DESC, created_at`, appID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	out := []DomainRow{}
	for rows.Next() {
		var d DomainRow
		if scanErr := rows.Scan(&d.ID, &d.AppID, &d.Hostname, &d.Verified,
			&d.TXTChallenge, &d.CNameTarget, &d.IsDefault); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RequireVerifiedDomain enforces the RB-25 rule that only verified domains
// may be bound to routing; unverified hosts surface as ErrConflict (409).
func (s *Store) RequireVerifiedDomain(ctx context.Context, orgID, appID, domainID string) (DomainRow, error) {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return DomainRow{}, getAppErr
	}
	d, domainErr := s.getDomain(ctx, appID, domainID)
	if domainErr != nil {
		return DomainRow{}, domainErr
	}
	if !d.Verified {
		return DomainRow{}, fmt.Errorf("%w: domain not verified", ErrConflict)
	}
	return d, nil
}

// VerifyCustomDomain checks the TXT challenge via the configured resolver and
// flips the domain to verified on a match; otherwise it stays pending.
func (s *Store) VerifyCustomDomain(
	ctx context.Context, resolver DNSResolver, appID, domainID string,
) (DomainRow, error) {
	d, domainErr := s.getDomain(ctx, appID, domainID)
	if domainErr != nil {
		return DomainRow{}, domainErr
	}
	if d.Verified {
		return d, nil
	}
	if d.TXTChallenge == "" {
		return DomainRow{}, fmt.Errorf("%w: domain has no active challenge", ErrConflict)
	}
	records, lookupErr := resolver.LookupTXT(ctx, challengePrefix+d.Hostname)
	if lookupErr == nil {
		for _, rec := range records {
			if rec == d.TXTChallenge {
				if markErr := s.markDomainVerified(ctx, d.ID); markErr != nil {
					return DomainRow{}, markErr
				}
				return s.getDomain(ctx, appID, d.ID)
			}
		}
	}
	return d, nil // challenge not found yet; stays pending
}
