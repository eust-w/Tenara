package auth

import (
	"net/http"

	"tenara/control-plane/internal/rbac"
)

// Bridge exposes the middleware stack to sibling packages (apps/mcp/web)
// without leaking unexported internals.
type Bridge struct{ S *Service }

func NewBridge(s *Service) Bridge { return Bridge{S: s} }

// Authenticated resolves JWT or API-token bearers into Identity context.
func (b Bridge) Authenticated(next http.HandlerFunc) http.HandlerFunc {
	return b.S.authenticated(next)
}

// RequireCap gates a route behind one capability of the caller role.
func (b Bridge) RequireCap(needed rbac.Capability, next http.HandlerFunc) http.HandlerFunc {
	return b.S.requireCap(needed, next)
}

// Idem enforces RB-33 replay semantics on mutations.
func (b Bridge) Idem(next http.HandlerFunc) http.HandlerFunc {
	return b.S.idem(next)
}

// Audited records an audit_logs row per completed mutation (RB-32).
func (b Bridge) Audited(action string, next http.HandlerFunc) http.HandlerFunc {
	return b.S.audited(action, next)
}

// IdentityFrom returns the verified caller scope.
func (b Bridge) IdentityFrom(r *http.Request) (userID, orgID string, ok bool) {
	id, idOK := identityFrom(r)
	if !idOK || id == nil {
		return "", "", false
	}
	return id.UserID, id.OrgID, true
}
