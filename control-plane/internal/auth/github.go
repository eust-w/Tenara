package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"tenara/control-plane/internal/kms"
)

const (
	githubScope       = "repo,read:user"
	oauthStateTTL     = 10 * time.Minute
	oauthAppTokenType = "oauth_app" //nolint:gosec // role label, not a credential
)

var ErrStateMismatch = errors.New("oauth state mismatch")

type GitHubOAuth struct {
	Store        *Store
	KMS          *kms.Stub
	ClientID     string
	ClientSecret string
	AuthBaseURL  string // e.g. https://github.com (mockable)
	APIBaseURL   string // e.g. https://api.github.com (mockable)
	RedirectURL  string // our /v1/github/callback absolute URL
}

func (g *GitHubOAuth) Begin(ctx context.Context, userID string) (authorizeURL, state string, err error) {
	state, err = RandomToken()
	if err != nil {
		return "", "", fmt.Errorf("gen state: %w", err)
	}
	if _, execErr := g.Store.pool.Exec(ctx,
		`INSERT INTO oauth_states (state, user_id) VALUES ($1, $2)`, state, userID); execErr != nil {
		return "", "", fmt.Errorf("store state: %w", execErr)
	}
	q := url.Values{
		"client_id":    {g.ClientID},
		"scope":        {githubScope},
		"state":        {state},
		"redirect_uri": {g.RedirectURL},
	}
	return g.AuthBaseURL + "/login/oauth/authorize?" + q.Encode(), state, nil
}

type ghTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Exchange consumes the state and swaps the code for an access token.
// The bound user is recovered from the state row.
func (g *GitHubOAuth) Exchange(ctx context.Context, code, state string) (accessToken, userID string, err error) {
	var boundUser string
	scanErr := g.Store.pool.QueryRow(ctx,
		// TTL mirrors oauthStateTTL in Go (10 minutes).
		`DELETE FROM oauth_states WHERE state = $1 AND created_at > now() - interval '10 minutes' RETURNING user_id`,
		state).Scan(&boundUser)
	if errors.Is(scanErr, pgx.ErrNoRows) {
		return "", "", ErrStateMismatch
	}
	if scanErr != nil {
		return "", "", fmt.Errorf("consume state: %w", scanErr)
	}

	form := url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.RedirectURL},
	}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost,
		g.AuthBaseURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if reqErr != nil {
		return "", "", reqErr
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		return "", "", fmt.Errorf("token request: %w", doErr)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("token endpoint status %d", res.StatusCode)
	}
	var parsed ghTokenResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&parsed); decodeErr != nil {
		return "", "", fmt.Errorf("decode token response: %w", decodeErr)
	}
	if parsed.AccessToken == "" {
		return "", "", errors.New("empty access token")
	}
	return parsed.AccessToken, boundUser, nil
}

func (g *GitHubOAuth) apiGet(ctx context.Context, path, token string) (*http.Response, error) {
	//nolint:gosec // G704: base URL is operator-configured (mock or GitHub), path is a constant
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, g.APIBaseURL+path, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	//nolint:gosec // G704: see above; destination is the configured GitHub API base
	return http.DefaultClient.Do(req)
}

func (g *GitHubOAuth) FetchLogin(ctx context.Context, token string) (string, error) {
	res, err := g.apiGet(ctx, "/user", token)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	var parsed struct {
		Login string `json:"login"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&parsed); decodeErr != nil {
		return "", decodeErr
	}
	return parsed.Login, nil
}

func (g *GitHubOAuth) ListRepos(ctx context.Context, token string, page int) (*http.Response, error) {
	path := fmt.Sprintf("/user/repos?page=%d&per_page=30&sort=updated", page)
	return g.apiGet(ctx, path, token)
}

// StoreBinding encrypts the GitHub token at rest (KMS-stub chain).
func (g *GitHubOAuth) StoreBinding(ctx context.Context, userID string, plainToken, username string) error {
	sealed, encErr := g.KMS.Encrypt([]byte(plainToken))
	if encErr != nil {
		return fmt.Errorf("encrypt token: %w", encErr)
	}
	_, execErr := g.Store.pool.Exec(ctx,
		`UPDATE users SET github_token_encrypted = $2, github_bound_at = now(),
		 github_username = $3, updated_at = now() WHERE id = $1`,
		userID, sealed, username)
	return execErr
}

func (g *GitHubOAuth) LoadToken(ctx context.Context, userID string) (string, error) {
	var sealed []byte
	scanErr := g.Store.pool.QueryRow(ctx,
		`SELECT github_token_encrypted FROM users WHERE id = $1`, userID).Scan(&sealed)
	if scanErr != nil {
		return "", scanErr
	}
	plain, decErr := g.KMS.Decrypt(sealed)
	if decErr != nil {
		return "", decErr
	}
	return string(plain), nil
}

func (g *GitHubOAuth) ClearBinding(ctx context.Context, userID string) error {
	_, execErr := g.Store.pool.Exec(ctx,
		`UPDATE users SET github_token_encrypted = NULL, github_bound_at = NULL,
		 github_username = NULL, updated_at = now() WHERE id = $1`, userID)
	return execErr
}

func (g *GitHubOAuth) TokenType() string { return oauthAppTokenType }
