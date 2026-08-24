package previews

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// Commenter posts the preview URL back onto the source PR (mocked in tests;
// live mode requires the GitHub App installation token per design doc).
type Commenter interface {
	PostPreviewURL(ctx context.Context, repo string, prNumber int, previewURL string) error
}

// GHCommenter posts through the GitHub issues-comments REST API.
type GHCommenter struct { //nolint:fieldalignment // tiny transport config DTO
	// BaseURL overrides the API root in tests; empty uses github.com.
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *GHCommenter) PostPreviewURL(
	ctx context.Context, repo string, prNumber int, previewURL string,
) error {
	base := c.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	body, mErr := json.Marshal(map[string]string{
		"body": ":rocket: preview ready: " + previewURL +
			" (env `" + PreviewEnvName(prNumber) + "`)",
	})
	if mErr != nil {
		return fmt.Errorf("marshal comment: %w", mErr)
	}
	req, rErr := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/repos/"+repo+"/issues/"+strconv.Itoa(prNumber)+"/comments",
		bytes.NewReader(body))
	if rErr != nil {
		return fmt.Errorf("build request: %w", rErr)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, doErr := client.Do(req)
	if doErr != nil {
		return fmt.Errorf("%w: post comment: %w", ErrGHUnavailable, doErr)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // response fully consumed below
		}
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: comment status %d", ErrGHUnavailable, resp.StatusCode)
	}
	return nil
}

// ErrGHUnavailable marks GitHub API transport failures.
var ErrGHUnavailable = errors.New("github api unavailable")
