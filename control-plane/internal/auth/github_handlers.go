package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type GithubHandlers struct {
	GitHub *GitHubOAuth
	Tokens *TokenManager
}

// Mount registers GitHub binding routes (contract paths use /v1/github/*).
func (h *GithubHandlers) Mount(r *chi.Mux) {
	r.Get("/v1/github/start", h.authenticated(h.handleStart))
	r.Get("/v1/github/callback", h.authenticated(h.handleCallback))
	r.Get("/v1/github/repos", h.authenticated(h.handleRepos))
	r.Delete("/v1/github/binding", h.authenticated(h.handleUnbind))
}

var errNoBinding = errors.New("github not bound")

func (h *GithubHandlers) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if len(token) < 8 || token[:7] != "Bearer " {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "bearer required")
			return
		}
		if _, parseErr := h.Tokens.Parse(token[7:]); parseErr != nil {
			writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}
		next(w, r)
	}
}

func (h *GithubHandlers) handleStart(w http.ResponseWriter, r *http.Request) {
	bearer := r.Header.Get("Authorization")[7:]
	userID, parseErr := h.Tokens.Parse(bearer)
	if parseErr != nil {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
		return
	}
	authorizeURL, _, beginErr := h.GitHub.Begin(r.Context(), userID)
	if beginErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "oauth start failed")
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *GithubHandlers) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	token, boundUser, exchangeErr := h.GitHub.Exchange(r.Context(), code, state)
	if exchangeErr != nil {
		if errors.Is(exchangeErr, ErrStateMismatch) {
			writeProblem(w, http.StatusForbidden, "FORBIDDEN", "state mismatch")
			return
		}
		writeProblem(w, http.StatusBadGateway, "UPSTREAM", "token exchange failed")
		return
	}
	username, userErr := h.GitHub.FetchLogin(r.Context(), token)
	if userErr != nil {
		writeProblem(w, http.StatusBadGateway, "UPSTREAM", "fetch user failed")
		return
	}
	if storeErr := h.GitHub.StoreBinding(r.Context(), boundUser, token, username); storeErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "store binding failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "bound", "login": username})
}

func (h *GithubHandlers) userIDFromBearer(r *http.Request) (string, bool) {
	bearer := r.Header.Get("Authorization")
	if len(bearer) < 8 {
		return "", false
	}
	userID, parseErr := h.Tokens.Parse(bearer[7:])
	return userID, parseErr == nil
}

func (h *GithubHandlers) handleRepos(w http.ResponseWriter, r *http.Request) {
	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		if parsed, parseErr := strconv.Atoi(pageParam); parseErr == nil && parsed > 0 {
			page = parsed
		}
	}
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
		return
	}
	token, loadErr := h.GitHub.LoadToken(r.Context(), userID)
	if loadErr != nil {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", errNoBinding.Error())
		return
	}
	res, listErr := h.GitHub.ListRepos(r.Context(), token, page)
	if listErr != nil {
		writeProblem(w, http.StatusBadGateway, "UPSTREAM", "repos fetch failed")
		return
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil || res.StatusCode != http.StatusOK {
		writeProblem(w, http.StatusBadGateway, "UPSTREAM", "repos fetch failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, writeErr := w.Write(body); writeErr != nil {
		return
	}
}

func (h *GithubHandlers) handleUnbind(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
		return
	}
	if clearErr := h.GitHub.ClearBinding(r.Context(), userID); clearErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "unbind failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
