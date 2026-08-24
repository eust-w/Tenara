// Package logs implements org-scoped log queries over Loki (RB§31). The
// app_id label filter is mandatory at construction time — a platform-wide or
// cross-app query cannot be expressed through this API at all.
package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Source narrows log streams to one pipeline.
type Source string

const (
	SourceBuild Source = "build"
	SourceApp   Source = "app"
)

func (s Source) valid() bool {
	return s == SourceBuild || s == SourceApp
}

// Query is a fully scoped log request. Org isolation is enforced by the
// caller (the app must belong to the requesting org before this query may be
// built); cross-org attempts are answered with 404 upstream of this package.
type Query struct {
	AppID  string
	Source Source
	Limit  int
}

const defaultLimit = 200

func (q Query) selector() string {
	return fmt.Sprintf(`{source=%q, app_id=%q}`, string(q.Source), q.AppID)
}

// BuildURL renders the Loki range-query endpoint for the scoped selector.
func (q Query) BuildURL(lokiBase string) (string, error) {
	if q.AppID == "" {
		return "", errors.New("refusing log query without app scope")
	}
	if !q.Source.valid() {
		return "", fmt.Errorf("invalid log source %q", q.Source)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	params := url.Values{}
	params.Set("query", q.selector())
	params.Set("limit", strconv.Itoa(limit))
	params.Set("direction", "backward")
	return strings.TrimSuffix(strings.TrimSuffix(lokiBase, "/"), "/") +
		"/loki/api/v1/query_range?" + params.Encode(), nil
}

// LogLine is one normalized log entry with its nanosecond timestamp.
type LogLine struct {
	Text          string
	TimestampNano int64
}

// ParseLokiResponse flattens Loki's stream/value pairs into ordered lines.
func ParseLokiResponse(body []byte) ([]LogLine, error) {
	var generic struct {
		Data struct {
			Result []struct {
				Values [][]any `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &generic); err != nil {
		return nil, fmt.Errorf("parse loki response: %w", err)
	}
	lines := make([]LogLine, 0, 64)
	for _, stream := range generic.Data.Result {
		for _, pair := range stream.Values {
			line := LogLine{}
			if len(pair) == 2 {
				if ts, ok := pair[0].(string); ok {
					line.TimestampNano, _ = strconv.ParseInt(ts, 10, 64)
				}
				if text, ok := pair[1].(string); ok {
					line.Text = text
				}
			}
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// Fetch executes the built query against Loki and returns normalized lines.
func Fetch(ctx context.Context, client *http.Client, lokiURL string) ([]LogLine, error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, lokiURL, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("build loki request: %w", reqErr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loki unreachable: %w", err)
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil {
			_ = cErr //nolint:staticcheck // body fully consumed; close error is benign
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki status %d", resp.StatusCode)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read loki body: %w", readErr)
	}
	return ParseLokiResponse(body)
}
