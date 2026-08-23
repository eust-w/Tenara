package apps

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidDigest      = errors.New("invalid image digest")
	ErrNotEnoughRevisions = errors.New("not enough revisions for rollback")
)

var digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type RevisionRow struct {
	Revision       int    `json:"revision"`
	GitSHA         string `json:"git_sha,omitempty"`
	BuildID        string `json:"build_id,omitempty"`
	ImageDigest    string `json:"image_digest"`
	ConfigVersion  int    `json:"config_version"`
	SecretRevision int    `json:"secret_revision"`
	AppSpecVersion string `json:"appspec_version"`
	CreatedAt      string `json:"created_at"`
}

type RevisionInput struct {
	GitSHA         string
	BuildID        string // nullable uuid
	ImageDigest    string
	ConfigVersion  int
	SecretRevision int
	AppSpecVersion string
}

const cols = `revision, COALESCE(git_sha,''), COALESCE(build_id::text,''), image_digest,
	config_version, secret_revision, appspec_version, created_at::text`

func scanRevision(row pgx.Row) (RevisionRow, error) {
	var r RevisionRow
	err := row.Scan(&r.Revision, &r.GitSHA, &r.BuildID, &r.ImageDigest,
		&r.ConfigVersion, &r.SecretRevision, &r.AppSpecVersion, &r.CreatedAt)
	return r, err
}

// SaveRevision appends the next sequential revision for a deployment.
// The RB-26 digest constraint is enforced by the schema CHECK and mirrored
// here for typed errors.
func (s *Store) SaveRevision(
	ctx context.Context, orgID, appID, deploymentID string, in RevisionInput,
) (RevisionRow, error) {
	if _, getAppErr := s.GetApp(ctx, orgID, appID); getAppErr != nil {
		return RevisionRow{}, getAppErr
	}
	if !digestRE.MatchString(in.ImageDigest) {
		return RevisionRow{}, fmt.Errorf("%w: %q", ErrInvalidDigest, in.ImageDigest)
	}
	if !validUUID(deploymentID) {
		return RevisionRow{}, ErrNotFound
	}
	var (
		next       int
		buildIDArg any
	)
	if in.BuildID != "" {
		buildIDArg = in.BuildID
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(max(revision), 0) + 1 FROM deployment_revisions
		 WHERE deployment_id = $1`, deploymentID).Scan(&next); err != nil {
		return RevisionRow{}, err
	}
	r, scanErr := scanRevision(s.pool.QueryRow(ctx,
		`INSERT INTO deployment_revisions
		 (deployment_id, revision, git_sha, build_id, image_digest,
		  config_version, secret_revision, appspec_version)
		 VALUES ($1::uuid, $2, NULLIF($3,''), $4, $5, $6, $7, $8)
		 RETURNING `+cols,
		deploymentID, next, in.GitSHA, buildIDArg, in.ImageDigest,
		in.ConfigVersion, in.SecretRevision, in.AppSpecVersion))
	return r, scanErr
}

// revColsPrefixed qualifies revision columns inside join queries.
const revColsPrefixed = `dr.revision, COALESCE(dr.git_sha,''), COALESCE(dr.build_id::text,''), dr.image_digest,
	dr.config_version, dr.secret_revision, dr.appspec_version, dr.created_at::text`

// RollbackTarget selects the second-newest revision (RB-26 R1): the newest is
// the live release, so rollback aims one step back.
func (s *Store) RollbackTarget(ctx context.Context, orgID, appID, deploymentID string) (RevisionRow, error) {
	if !validUUID(deploymentID) {
		return RevisionRow{}, ErrNotFound
	}
	r, scanErr := scanRevision(s.pool.QueryRow(ctx,
		`SELECT `+revColsPrefixed+` FROM deployment_revisions dr
		 JOIN deployments dp ON dp.id = dr.deployment_id
		 JOIN applications ap ON ap.id = dp.app_id
		 WHERE dr.deployment_id = $1 AND ap.org_id = $2 AND ap.id = $3
		 ORDER BY dr.revision DESC
		 LIMIT 1 OFFSET 1`,
		deploymentID, orgID, appID))
	if errors.Is(scanErr, pgx.ErrNoRows) {
		return RevisionRow{}, fmt.Errorf("%w: need at least two revisions", ErrNotEnoughRevisions)
	}
	return r, scanErr
}
