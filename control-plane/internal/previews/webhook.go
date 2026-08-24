// Package previews implements the P2 preview-deployment face (D2-P2-3 /
// todo90): GitHub pull_request webhooks drive preview-pr-{n} environments
// with signature verification, replay protection, URL comment backfill and
// soft-delete teardown reusing the todo54 pipeline.
package previews

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// EnvPrefix namespaces every preview environment (RB§31 naming).
	EnvPrefix = "preview-pr-"

	maxPayloadBytes = 1 << 20 // 1MiB webhook cap
)

// PreviewEnvName maps a PR number onto its dedicated preview env name.
func PreviewEnvName(prNumber int) string {
	return EnvPrefix + strconv.Itoa(prNumber)
}

// PREventAction is the reduced decision over a pull_request webhook event.
type PREventAction string

const (
	ActionOpen   PREventAction = "open"
	ActionClose  PREventAction = "close"
	ActionIgnore PREventAction = "ignore"
)

// DecidePRAction folds GitHub actions onto preview lifecycle moves: opened /
// reopened create the env; closed (merged or not) tears it down; everything
// else is ignored.
func DecidePRAction(action string, merged bool) PREventAction {
	switch action {
	case "opened", "reopened":
		return ActionOpen
	case "closed":
		_ = merged // both flavors tear down per design doc
		return ActionClose
	default:
		return ActionIgnore
	}
}

// VerifySignature checks the X-Hub-Signature-256 HMAC-SHA256 tag in
// constant time.
func VerifySignature(secret string, payload []byte, sigHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, writeErr := mac.Write(payload); writeErr != nil {
		return false
	}
	want := prefix + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(sigHeader))
}

// ReplayGuard rejects repeated delivery ids within a TTL window.
type ReplayGuard struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

// NewReplayGuard keeps delivery ids for ttl before pruning.
func NewReplayGuard(ttl time.Duration) *ReplayGuard {
	return &ReplayGuard{seen: map[string]time.Time{}, ttl: ttl}
}

// Allow records delivery and reports whether it is fresh.
func (g *ReplayGuard) Allow(delivery string, now time.Time) bool {
	if delivery == "" {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, ts := range g.seen {
		if now.Sub(ts) > g.ttl {
			delete(g.seen, id)
		}
	}
	if _, dup := g.seen[delivery]; dup {
		return false
	}
	g.seen[delivery] = now
	return true
}

// Hooks are the platform side effects triggered by the webhook.
type Hooks struct {
	OnOpen  func(prNumber int) error
	OnClose func(prNumber int) error
}

// Handler serves POST /v1/webhooks/github.
type Handler struct {
	Secret string
	Guard  *ReplayGuard
	Hooks  Hooks
}

type prPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest *struct {
		Merged bool `json:"merged"`
	} `json:"pull_request"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadBytes))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if !VerifySignature(h.Secret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if !h.Guard.Allow(r.Header.Get("X-GitHub-Delivery"), time.Now()) {
		http.Error(w, "replayed delivery", http.StatusConflict)
		return
	}
	var p prPayload
	if json.Unmarshal(body, &p) != nil || p.PullRequest == nil || p.Number <= 0 {
		http.Error(w, "unsupported payload", http.StatusBadRequest)
		return
	}
	switch DecidePRAction(p.Action, p.PullRequest.Merged) {
	case ActionOpen:
		if h.Hooks.OnOpen != nil {
			if hookErr := h.Hooks.OnOpen(p.Number); hookErr != nil {
				http.Error(w, "preview create failed", http.StatusInternalServerError)
				return
			}
		}
	case ActionClose:
		if h.Hooks.OnClose != nil {
			if hookErr := h.Hooks.OnClose(p.Number); hookErr != nil {
				http.Error(w, "preview teardown failed", http.StatusInternalServerError)
				return
			}
		}
	case ActionIgnore:
		// synchronize/labeled/etc.: no preview lifecycle move.
	}
	w.WriteHeader(http.StatusAccepted)
}
