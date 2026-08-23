package httpapi

import (
	"encoding/json"
	"net/http"

	"tenara/control-plane/internal/gen"
)

// Stub is the plan-todo-5 placeholder implementation: every operation answers
// 501 with an RFC7807 problem until its wave lands.
type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func Handler() http.Handler { return gen.Handler(&Stub{}) }

func (s *Stub) notImplemented(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(gen.Problem{
		Type:   "about:blank",
		Title:  "Not Implemented",
		Status: http.StatusNotImplemented,
	})
}

func (s *Stub) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) GetReadyz(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AdminApps(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) PutV1AdminQuota(w http.ResponseWriter, _ *http.Request, _ gen.PutV1AdminQuotaParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AdminSecurityEvents(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AdminUsers(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AdminUsersSuspend(
	w http.ResponseWriter,
	_ *http.Request,
	_ string,
	_ gen.PostV1AdminUsersSuspendParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PostV1Analyze(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AnalyzeParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1Apps(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) PostV1Apps(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AppsParams) {
	s.notImplemented(w)
}

func (s *Stub) DeleteV1AppsByAppId(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.DeleteV1AppsByAppIdParams,
) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsByAppId(w http.ResponseWriter, _ *http.Request, _ gen.AppId) {
	s.notImplemented(w)
}

func (s *Stub) PatchV1AppsByAppId(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PatchV1AppsByAppIdParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsDatabases(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.PostV1AppsDatabasesParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsDeployments(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.PostV1AppsDeploymentsParams,
) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsDeployments(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.DeploymentId) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsDiagnostics(w http.ResponseWriter, _ *http.Request, _ gen.AppId) {
	s.notImplemented(w)
}
