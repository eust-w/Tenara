package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Service struct {
	store   *Store
	tokens  *TokenManager
	baseURL string
}

func NewService(store *Store, tokens *TokenManager, baseURL string) *Service {
	return &Service{store: store, tokens: tokens, baseURL: baseURL}
}

// Mount registers the auth routes; they take precedence over the generated
// contract stubs because the parent router matches exact patterns first.
func (s *Service) Mount(r chi.Router) {
	r.Post("/v1/auth/register", s.handleRegister)
	r.Post("/v1/auth/verify", s.handleVerify)
	r.Post("/v1/auth/login", s.handleLogin)
	r.Post("/v1/auth/request-password-reset", s.handleRequestReset)
	r.Post("/v1/auth/reset-password", s.handleResetPassword)
	r.Get("/v1/me", s.authenticated(s.handleMe))
	r.Post("/v1/tokens", s.authenticated(s.handleCreateToken))
	r.Get("/v1/tokens", s.authenticated(s.handleListTokens))
	r.Delete("/v1/tokens/{tokenId}", s.authenticated(s.handleRevokeToken))
}

type ctxKey string

const identityKey ctxKey = "identity"

// Identity is the authenticated caller context carried through handlers.
type Identity struct {
	UserID    string
	OrgID     string
	ActorType string // RB-32 actor_type: user|agent|admin|controller
}

// resolveBearer accepts either a session JWT or an organization-scoped API
// token (tenara_ prefix) and resolves both to a full Identity.
func (s *Service) resolveBearer(r *http.Request) (*Identity, bool) {
	authz := r.Header.Get("Authorization")
	if len(authz) < 7 || authz[:7] != "Bearer " {
		return nil, false
	}
	raw := authz[7:]
	if strings.HasPrefix(raw, apiTokenPrefix) {
		userID, orgID, resolveErr := s.store.ResolveAPIToken(r.Context(), raw)
		if resolveErr != nil {
			return nil, false
		}
		return &Identity{UserID: userID, OrgID: orgID, ActorType: "user"}, true
	}
	userID, parseErr := s.tokens.Parse(raw)
	if parseErr != nil {
		return nil, false
	}
	orgID, orgErr := s.store.DefaultOrgForUser(r.Context(), userID)
	if orgErr != nil {
		return nil, false
	}
	return &Identity{UserID: userID, OrgID: orgID, ActorType: "user"}, true
}

func (s *Service) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := s.resolveBearer(r)
		if !ok {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), identityKey, ident)))
	}
}

func identityFrom(r *http.Request) (*Identity, bool) {
	ident, ok := r.Context().Value(identityKey).(*Identity)
	return ident, ok && ident != nil
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": ident.UserID, "org_id": ident.OrgID})
}

type registerInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func writeProblem(w http.ResponseWriter, status int, code string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(status),
		"status": status, "error_code": code, "detail": detail,
	})
}

func (s *Service) clientIP(r *http.Request) string {
	// Dev/test clients identify themselves via X-Forwarded-For; in production
	// the edge proxy overwrites this header before it reaches us.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
	if splitErr != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in registerInput
	if jsonErr := json.NewDecoder(r.Body).Decode(&in); jsonErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	addr, parseErr := mail.ParseAddress(in.Email)
	if parseErr != nil || !mailAddressHasLocalAndDomain(in.Email) {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid email")
		return
	}
	if passErr := ValidatePassword(in.Password); passErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", passErr.Error())
		return
	}
	if limitErr := s.store.CheckSignupLimit(r.Context(), s.clientIP(r)); limitErr != nil {
		if errors.Is(limitErr, ErrRateLimited) {
			writeProblem(w, http.StatusTooManyRequests, "RATE_LIMITED", "signup rate limit exceeded")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "storage error")
		return
	}

	hash, hashErr := HashPassword(in.Password)
	if hashErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "hash error")
		return
	}
	user, createErr := s.store.CreateUnverified(r.Context(), in.Email, hash)
	if createErr != nil {
		if errors.Is(createErr, ErrEmailTaken) {
			// Do not leak account existence; pretend success.
			w.WriteHeader(http.StatusCreated)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "create failed")
		return
	}

	token, tokenErr := RandomToken()
	if tokenErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "token error")
		return
	}
	if storeErr := s.store.StoreVerification(r.Context(), user.ID, hashToken(token)); storeErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "store verification failed")
		return
	}
	verifyURL := fmt.Sprintf("%s/v1/auth/verify?token=%s", s.baseURL, token)
	sender := SMTPSender{Addr: "127.0.0.1:1025"}
	if sendErr := sender.SendVerification(*addr, verifyURL); sendErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "mail send failed")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func mailAddressHasLocalAndDomain(addr string) bool {
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return false
	}
	at := -1
	for i := len(parsed.Address) - 1; i >= 0; i-- {
		if parsed.Address[i] == '@' {
			at = i
			break
		}
	}
	return at > 0 && at < len(parsed.Address)-1
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type verifyInput struct {
	Token string `json:"token"`
}

func (s *Service) handleVerify(w http.ResponseWriter, r *http.Request) {
	var in verifyInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil || in.Token == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "token required")
		return
	}
	if _, consumeErr := s.store.ConsumeVerification(r.Context(), hashToken(in.Token)); consumeErr != nil {
		writeProblem(w, http.StatusGone, "TOKEN_EXPIRED", "verification link invalid or expired")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in loginInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	user, getErr := s.store.GetByEmail(r.Context(), in.Email)
	if getErr != nil {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}
	ok, verifyErr := VerifyPassword(in.Password, user.PasswordHash)
	if verifyErr != nil || !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}
	if !user.EmailVerified {
		writeProblem(w, http.StatusForbidden, "FORBIDDEN", "email not verified")
		return
	}
	access, accessErr := s.tokens.NewAccess(user.ID)
	refresh, refreshErr := s.tokens.NewRefresh(user.ID) // rotation: fresh refresh each login
	if accessErr != nil || refreshErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "token error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"access_token": access, "refresh_token": refresh,
	})
}

type resetRequestInput struct {
	Email string `json:"email"`
}

func (s *Service) handleRequestReset(w http.ResponseWriter, r *http.Request) {
	var in resetRequestInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	user, getErr := s.store.GetByEmail(r.Context(), in.Email)
	if getErr == nil && user.EmailVerified {
		s.notifyPasswordReset(r.Context(), user)
	}
	// Always 202 to avoid account enumeration.
	w.WriteHeader(http.StatusAccepted)
}

func (s *Service) notifyPasswordReset(ctx context.Context, user User) {
	token, tokenErr := RandomToken()
	if tokenErr != nil {
		return
	}
	storeErr := s.store.StoreResetToken(ctx, user.ID, hashToken(token))
	if storeErr != nil {
		return
	}
	resetURL := fmt.Sprintf("%s/v1/auth/reset-password?token=%s", s.baseURL, token)
	// Best-effort notification; the caller always gets 202 regardless.
	sender := SMTPSender{Addr: "127.0.0.1:1025"}
	to, parseErr := mail.ParseAddress(user.Email)
	if parseErr != nil {
		return
	}
	sender.SendVerification(*to, resetURL) //nolint:errcheck,revive // fire-and-forget by design
}

type resetConfirmInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (s *Service) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var in resetConfirmInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid JSON")
		return
	}
	if passErr := ValidatePassword(in.NewPassword); passErr != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", passErr.Error())
		return
	}
	hash, hashErr := HashPassword(in.NewPassword)
	if hashErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "hash error")
		return
	}
	if consumeErr := s.store.ConsumeResetToken(r.Context(), hashToken(in.Token), hash); consumeErr != nil {
		writeProblem(w, http.StatusGone, "TOKEN_EXPIRED", "reset link invalid or expired")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
