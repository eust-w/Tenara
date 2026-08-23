package httpapi

import (
	"net/http"

	"tenara/control-plane/internal/gen"
)

func (s *Stub) GetV1AppsDomains(w http.ResponseWriter, _ *http.Request, _ gen.AppId) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsAppIdDomains(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.PostV1AppsAppIdDomainsParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsDomainsVerify(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ string,
	_ gen.PostV1AppsDomainsVerifyParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PutV1AppsEnv(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PutV1AppsEnvParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsEnvironments(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.PostV1AppsEnvironmentsParams,
) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsLogs(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.GetV1AppsLogsParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsPlan(w http.ResponseWriter, _ *http.Request, _ gen.AppId) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsRestart(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PostV1AppsRestartParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsRollback(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PostV1AppsRollbackParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1AppsSecrets(w http.ResponseWriter, _ *http.Request, _ gen.AppId) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsSecretsReveal(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.AppId,
	_ gen.PostV1AppsSecretsRevealParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AppsServices(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PostV1AppsServicesParams) {
	s.notImplemented(w)
}

func (s *Stub) PutV1AppsSpec(w http.ResponseWriter, _ *http.Request, _ gen.AppId, _ gen.PutV1AppsSpecParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1Auditlogs(w http.ResponseWriter, _ *http.Request, _ gen.GetV1AuditlogsParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AuthLogin(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AuthLoginParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AuthRegister(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AuthRegisterParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AuthRequestPasswordReset(
	w http.ResponseWriter,
	_ *http.Request,
	_ gen.PostV1AuthRequestPasswordResetParams,
) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AuthResetPassword(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AuthResetPasswordParams) {
	s.notImplemented(w)
}

func (s *Stub) PostV1AuthVerify(w http.ResponseWriter, _ *http.Request, _ gen.PostV1AuthVerifyParams) {
	s.notImplemented(w)
}

func (s *Stub) DeleteV1GithubBinding(w http.ResponseWriter, _ *http.Request, _ gen.DeleteV1GithubBindingParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1GithubCallback(w http.ResponseWriter, _ *http.Request, _ gen.GetV1GithubCallbackParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1GithubRepos(w http.ResponseWriter, _ *http.Request, _ gen.GetV1GithubReposParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1GithubStart(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) GetV1Me(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) GetV1Members(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) PostV1Members(w http.ResponseWriter, _ *http.Request, _ gen.PostV1MembersParams) {
	s.notImplemented(w)
}

func (s *Stub) GetV1Tokens(w http.ResponseWriter, _ *http.Request) {
	s.notImplemented(w)
}

func (s *Stub) PostV1Tokens(w http.ResponseWriter, _ *http.Request, _ gen.PostV1TokensParams) {
	s.notImplemented(w)
}

func (s *Stub) DeleteV1Tokens(w http.ResponseWriter, _ *http.Request, _ string, _ gen.DeleteV1TokensParams) {
	s.notImplemented(w)
}
