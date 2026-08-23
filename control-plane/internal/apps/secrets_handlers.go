package apps

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"tenara/control-plane/internal/secrets"
)

func (h *Handlers) handlePutEnv(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	if _, getAppErr := h.store.GetApp(r.Context(), orgID, appID); getAppErr != nil {
		mapWriteErr(w, getAppErr)
		return
	}
	var in struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if !decodeInto(r, &in) || strings.TrimSpace(in.Name) == "" || in.Value == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED",
			"name and value required")
		return
	}
	if setErr := h.secrets.SetSecret(r.Context(), appID,
		strings.TrimSpace(in.Name), in.Value); setErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "secret storage failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	if _, getAppErr := h.store.GetApp(r.Context(), orgID, appID); getAppErr != nil {
		mapWriteErr(w, getAppErr)
		return
	}
	items, listErr := h.secrets.ListSecrets(r.Context(), appID)
	if listErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "list failed")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handlers) handleRevealSecret(w http.ResponseWriter, r *http.Request) {
	userID, orgID, identOK := h.gate.IdentityFrom(r)
	if !identOK {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	appID := chi.URLParam(r, "appId")
	if _, getAppErr := h.store.GetApp(r.Context(), orgID, appID); getAppErr != nil {
		mapWriteErr(w, getAppErr)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeInto(r, &in) || in.Name == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name required")
		return
	}
	plaintext, revealErr := h.secrets.Reveal(r.Context(), appID, in.Name)
	if revealErr != nil {
		if errors.Is(revealErr, secrets.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "NOT_FOUND", "unknown secret")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "reveal failed")
		return
	}
	//nolint:errcheck // best-effort telemetry; result=revealed pinned by acceptance
	_ = h.store.InsertAuditRow(r.Context(), AuditRow{
		ActorType:   "user",
		ActorID:     userID,
		WorkspaceID: orgID,
		Action:      "secret.reveal",
		Result:      "revealed",
	})
	writeJSON(w, http.StatusOK, map[string]string{"value": plaintext})
}
