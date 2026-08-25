package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tenara/control-plane/internal/appspec"
	"tenara/control-plane/internal/kms"
	"tenara/control-plane/internal/orchestrator/plan"
	"tenara/control-plane/internal/provision"
	"tenara/control-plane/internal/rbac"
	"tenara/control-plane/internal/secrets"
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
	// Analyze abstracts the repository analyzer via DI; the returned JSON
	// carries an "app_spec" member persisted as current_spec.
	Analyze func(repoPath, baseDomain string) (json.RawMessage, error)
	// Applier optionally materializes cluster objects (env-gated DI).
	Applier    provision.Applier
	store      *Store
	secrets    *secrets.Service //nolint:fieldalignment // handler singleton; layout irrelevant
	gate       Gate
	baseDomain string
	resolver   DNSResolver
}

func New(
	pool *pgxpool.Pool, gate Gate, baseDomain string, kmsStub *kms.Stub,
) *Handlers {
	return &Handlers{
		store: NewStore(pool), gate: gate,
		baseDomain: baseDomain, secrets: secrets.New(pool, kmsStub),
		resolver: net.DefaultResolver,
	}
}

// SetDNSResolver overrides the TXT lookup source (verification test seam).
func (h *Handlers) SetDNSResolver(r DNSResolver) { h.resolver = r }

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
	case errors.Is(err, ErrUnsupportedKind):
		writeProblem(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
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
	r.Post("/v1/apps/{appId}/databases", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapDatabaseCreate,
			h.gate.Idem(h.gate.Audited("database.create", h.handleRequestDatabase)))))
	r.Get("/v1/apps/{appId}/domains", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleListDomains)))
	r.Post("/v1/apps/{appId}/domains", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapDomainBind,
			h.gate.Idem(h.gate.Audited("domain.bind", h.handleAddDomain)))))
	r.Post("/v1/apps/{appId}/domains/{domainId}/verify", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapDomainBind, h.handleVerifyDomain)))
	r.Get("/v1/apps/{appId}/plan", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleGetPlan)))
	r.Post("/v1/apps/{appId}/analyze", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppCreate, h.handleAnalyze)))
	r.Post("/v1/apps/{appId}/deployments", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppDeploy,
			h.gate.Idem(h.gate.Audited("app.deploy", h.handleDeploy)))))
	r.Get("/v1/apps/{appId}/deployments", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleListDeployments)))
	r.Post("/v1/apps/{appId}/rollback", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapAppDeploy,
			h.gate.Idem(h.gate.Audited("app.rollback", h.handleRollback)))))
	r.Put("/v1/apps/{appId}/env", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapSecretWrite,
			h.gate.Idem(h.gate.Audited("secret.set", h.handlePutEnv)))))
	r.Get("/v1/apps/{appId}/secrets", h.gate.RequireCap(rbac.CapAppRead,
		h.gate.Authenticated(h.handleListSecrets)))
	r.Post("/v1/apps/{appId}/secrets/reveal", h.gate.Authenticated(
		h.gate.RequireCap(rbac.CapSecretReveal,
			h.handleRevealSecret)))
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

// handleRollback serves POST /v1/apps/{appId}/rollback (RB-26 R1).
func (h *Handlers) handleRollback(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	newRev, target, rbErr := h.store.RollbackLatest(r.Context(), orgID, chi.URLParam(r, "appId"))
	if rbErr != nil {
		if errors.Is(rbErr, ErrNotEnoughRevisions) {
			writeProblem(w, http.StatusConflict, "NOT_ENOUGH_REVISIONS", rbErr.Error())
			return
		}
		mapWriteErr(w, rbErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rolled_back_to": target.Revision,
		"new_revision":   newRev.Revision,
	})
}

// handleAnalyze runs the repository analyzer and persists the derived
// AppSpec as the app's current_spec override.

// handleListDeployments serves GET /v1/apps/{appId}/deployments.
func (h *Handlers) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	revs, listErr := h.store.ListRevisions(r.Context(), orgID, chi.URLParam(r, "appId"))
	if listErr != nil {
		mapWriteErr(w, listErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": revs})
}

func (h *Handlers) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	if h.Analyze == nil {
		writeProblem(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "analyzer not wired")
		return
	}
	var in struct {
		RepoPath string `json:"repo_path"`
	}
	if !decodeInto(r, &in) || strings.TrimSpace(in.RepoPath) == "" {
		writeProblem(w, http.StatusBadRequest, "BAD_REQUEST", "repo_path required")
		return
	}
	raw, analyzeErr := h.Analyze(in.RepoPath, h.baseDomain)
	if analyzeErr != nil {
		if errors.Is(analyzeErr, ErrUnsupportedStackKind) {
			writeProblem(w, http.StatusBadRequest, "UNSUPPORTED_STACK", analyzeErr.Error())
			return
		}
		mapWriteErr(w, fmt.Errorf("analyze: %w", analyzeErr))
		return
	}
	var envelope struct {
		AppSpec json.RawMessage `json:"app_spec"`
	}
	if unmarshalErr := json.Unmarshal(raw, &envelope); unmarshalErr != nil || len(envelope.AppSpec) == 0 {
		writeProblem(w, http.StatusUnprocessableEntity, "INVALID_SPEC", "analysis lacks app_spec")
		return
	}
	if _, parseErr := appspec.Parse(envelope.AppSpec); parseErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "INVALID_SPEC", parseErr.Error())
		return
	}
	appID := chi.URLParam(r, "appId")
	if saveErr := h.store.SaveAppSpec(r.Context(), orgID, appID, envelope.AppSpec); saveErr != nil {
		mapWriteErr(w, saveErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spec": envelope.AppSpec})
}

func (h *Handlers) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	appWithSpec, getErr := h.store.GetAppWithSpec(r.Context(), orgID, appID)
	if getErr != nil {
		mapWriteErr(w, getErr)
		return
	}
	if len(appWithSpec.SpecRaw) == 0 {
		writeProblem(w, http.StatusConflict, "SPEC_REQUIRED",
			"analyze or override a spec first")
		return
	}
	spec, parseErr := appspec.Parse(appWithSpec.SpecRaw)
	if parseErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "INVALID_SPEC", parseErr.Error())
		return
	}
	planOut, genErr := plan.Generate(plan.Input{
		AppID:      appID,
		Slug:       appWithSpec.Slug,
		Env:        "production",
		BaseDomain: h.baseDomain,
		Spec:       spec,
		Now:        time.Now(),
		TTL:        24 * time.Hour,
	})
	if genErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", genErr.Error())
		return
	}
	snapshot, marshalErr := json.Marshal(planOut)
	if marshalErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "snapshot failed")
		return
	}
	_, planID, saveErr := h.store.SaveAwaitingPlan(
		r.Context(), orgID, appID, "production", snapshot)
	if saveErr != nil {
		mapWriteErr(w, saveErr)
		return
	}
	planOut.PlanID = planID
	writeJSON(w, http.StatusOK, planOut)
}

func (h *Handlers) handleDeploy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	var in struct {
		PlanID string `json:"plan_id"`
		Image  string `json:"image"`
		GitURL string `json:"git_url"`
		GitSHA string `json:"git_sha"`
	}
	if !decodeInto(r, &in) || in.PlanID == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "plan_id required")
		return
	}
	depID, approveErr := h.store.ApprovePlan(
		r.Context(), orgID, chi.URLParam(r, "appId"), in.PlanID)
	switch {
	case approveErr == nil:
	case errors.Is(approveErr, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "unknown plan")
		return
	case errors.Is(approveErr, ErrPlanExpired):
		writeProblem(w, http.StatusGone, "PLAN_EXPIRED", "plan approval window elapsed")
		return
	case errors.Is(approveErr, ErrConflict):
		writeProblem(w, http.StatusConflict, "CONFLICT", approveErr.Error())
		return
	default:
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "approval failed")
		return
	}
	h.applyBridgeBestEffort(r.Context(), orgID, chi.URLParam(r, "appId"), in.Image, in.GitURL, in.GitSHA)

	writeJSON(w, http.StatusAccepted, map[string]string{"id": depID, "state": "PLANNED"})
}

func (h *Handlers) handleListDomains(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	if _, getAppErr := h.store.GetApp(r.Context(), orgID, appID); getAppErr != nil {
		mapWriteErr(w, getAppErr)
		return
	}
	domains, listErr := h.store.listDomains(r.Context(), appID)
	if listErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "list failed")
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (h *Handlers) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	appRow, getErr := h.store.GetApp(r.Context(), orgID, appID)
	if getErr != nil {
		mapWriteErr(w, getErr)
		return
	}
	var in struct {
		Hostname string `json:"hostname"`
	}
	if !decodeInto(r, &in) {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	hostname := strings.ToLower(strings.TrimSpace(in.Hostname))
	var (
		domain DomainRow
		addErr error
	)
	if hostname == "" {
		domain, addErr = h.store.AllocateDefaultDomain(
			r.Context(), appID, appRow.Slug, h.baseDomain)
	} else {
		domain, addErr = h.store.AddCustomDomain(r.Context(), appID, hostname)
	}
	if addErr != nil {
		mapWriteErr(w, addErr)
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}

func (h *Handlers) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	appID := chi.URLParam(r, "appId")
	if _, getAppErr := h.store.GetApp(r.Context(), orgID, appID); getAppErr != nil {
		mapWriteErr(w, getAppErr)
		return
	}
	domain, verifyErr := h.store.VerifyCustomDomain(
		r.Context(), h.resolver, appID, chi.URLParam(r, "domainId"))
	if verifyErr != nil {
		mapWriteErr(w, verifyErr)
		return
	}
	writeJSON(w, http.StatusOK, domain)
}

func (h *Handlers) handleRequestDatabase(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgOrUnauthorized(w, r)
	if !ok {
		return
	}
	var in struct {
		Kind      string `json:"kind"`
		Isolation string `json:"isolation"`
	}
	_ = decodeInto(r, &in) // optional body; default mongo/shared
	db, binding, createErr := h.store.RequestDatabase(
		r.Context(), orgID, chi.URLParam(r, "appId"), in.Kind, in.Isolation)
	if createErr != nil {
		mapWriteErr(w, createErr)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"database": db, "binding": binding,
	})
}

// applyBridgeBestEffort materializes the app's AppEnv when a cluster
// bridge is configured; failures are logged and never fail the API call
// (convergence is eventual by design).
func (h *Handlers) applyBridgeBestEffort(ctx context.Context, orgID, appIDParam, image, gitURL, gitSHA string) {
	if h.Applier == nil {
		return
	}
	app, getAppErr := h.store.GetApp(ctx, orgID, appIDParam)
	if getAppErr != nil {
		return
	}
	tier, tierErr := h.store.OrgTier(ctx, orgID)
	if tierErr != nil {
		tier = ""
	}
	input := provision.AppEnvInput{
		AppID:     app.ID,
		Env:       "production",
		Name:      app.Slug,
		QuotaTier: tierOrFree(tier),
		Isolation: "shared",
	}
	switch {
	case image != "":
		if !strings.Contains(image, "@sha256:") {
			log.Printf("provision.skip non-digest image %q", image)
		} else {
			input.Services = []provision.ServiceInput{{Name: app.Slug + "-web", Image: image}}
		}
		obj := provision.BuildAppEnv(input)
		if applyErr := h.Applier.Apply(ctx, obj); applyErr != nil {
			log.Printf("provision.apply appenv app=%s: %v", app.Slug, applyErr)
		}
	case gitURL != "":
		name := fmt.Sprintf("%s-b-%d", app.Slug, time.Now().UnixNano()%100000)
		bObj := provision.BuildBuild(provision.BuildInput{
			AppID: app.ID, Env: "production", Name: name,
			GitURL: gitURL, GitSHA: gitSHA,
		})
		if applyErr := h.Applier.Apply(ctx, bObj); applyErr != nil {
			log.Printf("provision.apply build app=%s: %v", app.Slug, applyErr)
		}
	default:
		log.Printf("provision.skip: no image or git source")
	}
}
