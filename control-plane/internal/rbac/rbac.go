// Package rbac implements the Tenara capability model (RB-6 + R11):
// roles never grant permissions directly; every authorization decision goes
// through Can(role, capability) so handlers stay free of scattered checks.
package rbac

type Capability string

const (
	CapAppCreate         Capability = "app:create"
	CapAppRead           Capability = "app:read"
	CapAppDeploy         Capability = "app:deploy"
	CapAppRollback       Capability = "app:rollback"
	CapAppDelete         Capability = "app:delete"
	CapDatabaseCreate    Capability = "database:create"
	CapDatabaseDelete    Capability = "database:delete"
	CapSecretWrite       Capability = "secret:write"
	CapSecretReveal      Capability = "secret:reveal" // R11 gated reveal
	CapDomainBind        Capability = "domain:bind"
	CapMemberInvite      Capability = "member:invite"
	CapAdminUserManage   Capability = "admin:user:manage"
	CapAdminQuotaManage  Capability = "admin:quota:manage"
	CapAdminClusterRead  Capability = "admin:cluster:read"
	CapAdminSecurityRead Capability = "admin:security:read"
	CapBillingManage     Capability = "billing:manage"
)

type Role string

const (
	RolePlatformAdmin  Role = "platform_admin"
	RoleWorkspaceAdmin Role = "workspace_admin"
	RoleDeveloper      Role = "developer"
	RoleMember         Role = "member"
	RoleViewer         Role = "viewer"
	RoleBillingAdmin   Role = "billing_admin"
	RoleSecurityAdmin  Role = "security_admin"
)

// AllCapabilities lists every capability in RB order.
func AllCapabilities() []Capability {
	return []Capability{
		CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback, CapAppDelete,
		CapDatabaseCreate, CapDatabaseDelete,
		CapSecretWrite, CapSecretReveal,
		CapDomainBind,
		CapMemberInvite,
		CapAdminUserManage, CapAdminQuotaManage, CapAdminClusterRead, CapAdminSecurityRead,
		CapBillingManage,
	}
}

// AllRoles lists every role in descending privilege order.
func AllRoles() []Role {
	return []Role{
		RolePlatformAdmin, RoleWorkspaceAdmin, RoleDeveloper, RoleMember,
		RoleViewer, RoleBillingAdmin, RoleSecurityAdmin,
	}
}

var roleCapabilities = map[Role]map[Capability]struct{}{
	RolePlatformAdmin: set(AllCapabilities()...),
	RoleWorkspaceAdmin: set(
		CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback, CapAppDelete,
		CapDatabaseCreate, CapDatabaseDelete,
		CapSecretWrite, CapSecretReveal,
		CapDomainBind,
		CapMemberInvite,
	),
	RoleMember: set(
		CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback,
		CapDatabaseCreate,
		CapSecretWrite,
		CapDomainBind,
	),
	// developer ships and cleans up: member plus lifecycle deletions.
	RoleDeveloper: set(
		CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback, CapAppDelete,
		CapDatabaseCreate, CapDatabaseDelete,
		CapSecretWrite,
		CapDomainBind,
	),
	// viewer is strictly read-only observability.
	RoleViewer: set(
		CapAppRead,
	),
	// billing_admin owns spend: plan tiers and invoices, nothing else.
	RoleBillingAdmin: set(
		CapAppRead,
		CapBillingManage,
	),
	// security_admin reviews the incident surface; gated secret:reveal
	// stays with workspace admins.
	RoleSecurityAdmin: set(
		CapAppRead,
		CapAdminSecurityRead,
	),
}

func set(caps ...Capability) map[Capability]struct{} {
	m := make(map[Capability]struct{}, len(caps))
	for _, c := range caps {
		m[c] = struct{}{}
	}
	return m
}

// Can reports whether role holds the capability.
func Can(role Role, capability Capability) bool {
	_, ok := roleCapabilities[role][capability]
	return ok
}

// IsValidRole guards membership writes against unknown roles.
func IsValidRole(role Role) bool {
	_, ok := roleCapabilities[role]
	return ok
}
