package auth

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type createTokenInput struct {
	Name string `json:"name"`
}

func (s *Service) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	var in createTokenInput
	if decodeErr := json.NewDecoder(r.Body).Decode(&in); decodeErr != nil || in.Name == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name required")
		return
	}
	orgID, orgErr := s.store.DefaultOrgForUser(r.Context(), ident.UserID)
	if orgErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "org resolution failed")
		return
	}
	plaintext, prefix, hash, genErr := GenerateAPIToken()
	if genErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "token generation failed")
		return
	}
	id, createErr := s.store.CreateAPIToken(r.Context(), ident.UserID, orgID, in.Name, hash, prefix)
	if createErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "token persist failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// The plaintext leaves the system exactly once, in this response.
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": id, "name": in.Name, "prefix": prefix, "plaintext": plaintext,
	})
}

func (s *Service) handleListTokens(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	rows, listErr := s.store.ListAPITokens(r.Context(), ident.UserID)
	if listErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "list failed")
		return
	}
	if rows == nil {
		rows = []APITokenRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (s *Service) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	ident, ok := identityFrom(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity")
		return
	}
	tokenID := chi.URLParam(r, "tokenId")
	revoked, revokeErr := s.store.RevokeAPIToken(r.Context(), ident.UserID, tokenID)
	if revokeErr != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "revoke failed")
		return
	}
	if !revoked {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "token not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
