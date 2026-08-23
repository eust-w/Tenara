package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// AuditEntry is one mutation record (RB-32). Secrets never appear here:
// handlers pass only masked snapshots, and this layer stores what it gets.
type AuditEntry struct {
	ActorType   string
	ActorID     string
	Agent       string
	WorkspaceID string
	Action      string
	SourceIP    string
	RequestID   string
	Result      string
}

// DetectAgent attributes coding-agent callers by User-Agent (RB-32).
func DetectAgent(ua string) string {
	lower := strings.ToLower(ua)
	for _, a := range []string{"codex", "claude", "cursor"} {
		if strings.Contains(lower, a) {
			return a
		}
	}
	return ""
}

func (s *Store) InsertAudit(ctx context.Context, e AuditEntry) error {
	_, execErr := s.pool.Exec(ctx,
		`INSERT INTO audit_logs
		 (actor_type, actor_id, agent, workspace_id, action, source_ip, request_id, result)
		 VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,''), NULLIF($4,'')::uuid,
		         $5, NULLIF($6,'')::inet, NULLIF($7,''), $8)`,
		e.ActorType, e.ActorID, e.Agent, e.WorkspaceID,
		e.Action, e.SourceIP, e.RequestID, e.Result)
	return execErr
}

func (s *Store) InsertSecurityEvent(ctx context.Context, kind, sourceIP, detail string) error {
	_, execErr := s.pool.Exec(ctx,
		`INSERT INTO security_events (kind, source_ip, detail)
		 VALUES ($1, NULLIF($2,'')::inet, to_jsonb($3::text))`,
		kind, sourceIP, detail)
	return execErr
}

type AuditLogRow struct {
	ActorType string `json:"actor_type"`
	Agent     string `json:"agent"`
	Action    string `json:"action"`
	Result    string `json:"result"`
	Occurred  string `json:"occurred_at"`
}

func (s *Store) ListAudit(ctx context.Context, orgID, action string) ([]AuditLogRow, error) {
	rows, queryErr := s.pool.Query(ctx,
		`SELECT actor_type, COALESCE(agent,''), action, result, occurred_at::text
		 FROM audit_logs WHERE workspace_id = $1 AND ($2 = '' OR action = $2)
		 ORDER BY occurred_at DESC LIMIT 100`, orgID, action)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	out := []AuditLogRow{}
	for rows.Next() {
		var r AuditLogRow
		if scanErr := rows.Scan(&r.ActorType, &r.Agent, &r.Action, &r.Result, &r.Occurred); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// audited records one audit row per completed mutation. It must sit inside
// authenticated/requireCap so identity context exists when the handler ends.
func (s *Service) audited(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cw := &captureRW{ResponseWriter: w, status: http.StatusOK}
		next(cw, r)

		ident, identOK := identityFrom(r)
		if !identOK {
			return
		}
		agent := DetectAgent(r.UserAgent())
		actorType := ident.ActorType
		if agent != "" {
			actorType = "agent"
		}
		result := "success"
		if cw.status >= http.StatusBadRequest {
			result = http.StatusText(cw.status)
			if result == "" {
				result = "error"
			}
		}
		insertErr := s.store.InsertAudit(r.Context(), AuditEntry{
			ActorType:   actorType,
			ActorID:     ident.UserID,
			Agent:       agent,
			WorkspaceID: ident.OrgID,
			Action:      action,
			SourceIP:    s.clientIP(r),
			RequestID:   r.Header.Get("X-Request-Id"),
			Result:      result,
		})
		if insertErr != nil {
			//nolint:errcheck // audit failure must not fail the caller
			_ = insertErr
		}
	}
}

func (s *Service) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	rows, listErr := s.store.ListAudit(r.Context(), ident.OrgID, r.URL.Query().Get("action"))
	if listErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "audit query failed")
		return
	}
	if rows == nil {
		rows = []AuditLogRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}
