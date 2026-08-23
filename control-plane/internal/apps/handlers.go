package apps

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/appspec"
	"tenara/control-plane/internal/rbac"
)

// Gate is the middleware contract injected from the auth package.
type Gate interface {
	Authenticated(next http.HandlerFunc) http.HandlerFunc
	RequireCap(needed rbac.Capability, next http.HandlerFunc) http.HandlerFunc
	Idem(next http.HandlerFunc) http.HandlerFunc
	Audited(action string, next http.HandlerFunc) http.HandlerFunc
	IdentityFrom(r *http.Request) (userID, orgID string, ok bool)
}

type Handlers struct {
	store *Store
	gate  Gate
}

func New(pool *pgxpool.Pool, gate Gate) *Handlers {
	return &Handlers{store: NewStore(pool), gate: gate}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck // response encode; nothing actionable on failure
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	//nolint:errcheck // response encode; nothing actionable on failure
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(status),
		"status": status, "error_code": code, "detail": detail,
	})
}

func mapWriteErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "not found")
	case errors.Is(err, ErrQuotaExceeded):
		writeProblem(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", err.Error())
	case errors.Is(err, ErrConflict):
		writeProblem(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "storage error")
	}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Get("/v1/apps", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleList)))
	r.Post("/v1/apps", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate,
			h.gate.Idem(h.gate.Audited("app.create", h.handleCreate)))))
	r.Get("/v1/apps/{appId}", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleGet)))
	r.Patch("/v1/apps/{appId}", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate,
			h.gate.Idem(h.gate.Audited("app.update", h.handleRename)))))
	r.Delete("/v1/apps/{appId}", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppDelete,
			h.gate.Idem(h.gate.Audited("app.delete", h.handleDelete)))))
	r.Post("/v1/apps/{appId}/services", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate,
			h.gate.Idem(h.gate.Audited("service.create", h.handleAddService)))))
	r.Post("/v1/apps/{appId}/environments", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate,
			h.gate.Idem(h.gate.Audited("environment.create", h.handleAddEnvironment)))))
	r.Put("/v1/apps/{appId}/spec", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate,
			h.gate.Idem(h.gate.Audited("app.spec.override", h.handlePutSpec)))))
}

func (h *Handlers) orgOrUnauthorized(w http.ResponseWriter, r *http.Request) (string, bool) {
	_, orgID, ok := h.gate.IdentityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
	}
	return orgID, ok
}

func decodeInto(r *http.Request, target any) bool {
	return json.NewDecoder(r.Body).Decode(target) == nil
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	list, listErr := h.store.ListApps(r.Context(), orgID)
	if listErr != nil {
		mapWriteErr(w, listErr)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID, orgID, identOK := h.gate.IdentityFrom(r)
	if !identOK {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeInto(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name required")
		return
	}
	app, createErr := h.store.CreateApp(r.Context(), orgID,
		strings.TrimSpace(in.Name), userID)
	if createErr != nil {
		mapWriteErr(w, createErr)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	app, getErr := h.store.GetApp(r.Context(), orgID, chi.URLParam(r, "appId"))
	if getErr != nil {
		mapWriteErr(w, getErr)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handlers) handleRename(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeInto(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name required")
		return
	}
	app, renameErr := h.store.RenameApp(r.Context(), orgID,
		chi.URLParam(r, "appId"), strings.TrimSpace(in.Name))
	if renameErr != nil {
		mapWriteErr(w, renameErr)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	if deleteErr := h.store.SoftDeleteApp(r.Context(), orgID, chi.URLParam(r, "appId")); deleteErr != nil {
		mapWriteErr(w, deleteErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type serviceInput struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Runtime string `json:"runtime"`
	Port    int    `json:"port"`
}

func (h *Handlers) handleAddService(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	var in serviceInput
	if !decodeInto(r, &in) || strings.TrimSpace(in.Name) == "" ||
		(in.Type != "frontend" && in.Type != "backend") ||
		(in.Runtime != "node" && in.Runtime != "python" && in.Runtime != "go") ||
		in.Port < 0 || in.Port > 65535 {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED",
			"name/type/runtime/port invalid")
		return
	}
	svc, addErr := h.store.AddService(r.Context(), orgID,
		chi.URLParam(r, "appId"), in.Name, in.Type, in.Runtime, in.Port)
	if addErr != nil {
		mapWriteErr(w, addErr)
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *Handlers) handleAddEnvironment(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decodeInto(r, &in) {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	namespace, envErr := h.store.AddEnvironment(r.Context(), orgID,
		chi.URLParam(r, "appId"), in.Name)
	if envErr != nil {
		mapWriteErr(w, envErr)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"namespace_name": namespace})
}

func (h *Handlers) handlePutSpec(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	raw, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_FAILED", "unreadable body")
		return
	}
	if _, parseErr := appspec.Parse(raw); parseErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "INVALID_SPEC", parseErr.Error())
		return
	}
	if updateErr := h.store.UpdateSpec(r.Context(), orgID,
		chi.URLParam(r, "appId"), raw); updateErr != nil {
		mapWriteErr(w, updateErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
