package rbac

import "testing"

func TestCapabilityMatrix(t *testing.T) {
	cases := []struct {
		role Role
		cap  Capability
		want bool
	}{
		// member lacks destructive and administrative capabilities (acceptance)
		{RoleMember, CapAppDelete, false},
		{RoleMember, CapAdminUserManage, false},
		{RoleMember, CapSecretReveal, false},
		{RoleMember, CapMemberInvite, false},
		{RoleMember, CapAdminSecurityRead, false},
		// workspace_admin holds database:create but not admin:user:manage (acceptance pins this pair)
		{RoleWorkspaceAdmin, CapDatabaseCreate, true},
		{RoleWorkspaceAdmin, CapAdminUserManage, false},
		{RoleWorkspaceAdmin, CapAppDelete, true},
		{RoleWorkspaceAdmin, CapSecretReveal, true},
		{RoleWorkspaceAdmin, CapMemberInvite, true},
	}

	for _, tc := range cases {
		if got := Can(tc.role, tc.cap); got != tc.want {
			t.Errorf("Can(%s, %s) = %v, want %v", tc.role, tc.cap, got, tc.want)
		}
	}
}

func TestPlatformAdminPassesEverything(t *testing.T) {
	for _, c := range AllCapabilities() {
		if !Can(RolePlatformAdmin, c) {
			t.Errorf("platform_admin missing %s", c)
		}
	}
}

func TestUnknownRoleDenied(t *testing.T) {
	if Can(Role("superuser"), CapAppRead) {
		t.Fatal("unknown role must be denied")
	}
}

func mkSet(caps ...Capability) map[Capability]bool {
	m := map[Capability]bool{}
	for _, c := range caps {
		m[c] = true
	}
	return m
}

// TestMatrixFullCoverage asserts every role x capability cell against an
// independently written expectation grid (plan todo94 acceptance). The grid
// is 7x16: the plan sketched 14 columns before billing:manage was recognized
// as unavoidable for the billing_admin role; app:rollback/database:delete
// stay for API compatibility with shipped handlers.
func TestMatrixFullCoverage(t *testing.T) {
	all := mkSet(AllCapabilities()...)
	want := map[Role]map[Capability]bool{
		RolePlatformAdmin: all,
		RoleWorkspaceAdmin: mkSet(
			CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback, CapAppDelete,
			CapDatabaseCreate, CapDatabaseDelete,
			CapSecretWrite, CapSecretReveal,
			CapDomainBind, CapMemberInvite,
		),
		RoleDeveloper: mkSet(
			CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback, CapAppDelete,
			CapDatabaseCreate, CapDatabaseDelete,
			CapSecretWrite, CapDomainBind,
		),
		RoleMember: mkSet(
			CapAppCreate, CapAppRead, CapAppDeploy, CapAppRollback,
			CapDatabaseCreate, CapSecretWrite, CapDomainBind,
		),
		RoleViewer:        mkSet(CapAppRead),
		RoleBillingAdmin:  mkSet(CapAppRead, CapBillingManage),
		RoleSecurityAdmin: mkSet(CapAppRead, CapAdminSecurityRead),
	}

	roles := AllRoles()
	caps := AllCapabilities()
	if len(roles) != 7 || len(caps) != 16 {
		t.Fatalf("grid drifted: %d roles x %d caps", len(roles), len(caps))
	}
	for _, r := range roles {
		wr, ok := want[r]
		if !ok {
			t.Fatalf("expectation missing for role %s", r)
		}
		for _, c := range caps {
			if got, wantCell := Can(r, c), wr[c]; got != wantCell {
				t.Errorf("matrix(%s, %s) = %v, want %v", r, c, got, wantCell)
			}
		}
	}
}

func TestNewRolesLeastPrivilege(t *testing.T) {
	if Can(RoleViewer, CapAppDeploy) || Can(RoleViewer, CapSecretReadHint()) {
		t.Fatal("viewer must stay read-only")
	}
	if !Can(RoleBillingAdmin, CapBillingManage) {
		t.Fatal("billing_admin must manage billing")
	}
	if Can(RoleSecurityAdmin, CapSecretReveal) {
		t.Fatal("gated reveal stays out of security_admin")
	}
	if !Can(RoleDeveloper, CapAppDelete) || !Can(RoleDeveloper, CapDatabaseDelete) {
		t.Fatal("developer owns lifecycle cleanup")
	}
	if IsValidRole(Role("root")) {
		t.Fatal("unknown role must stay invalid")
	}
}

// CapSecretReadHint exists purely to prove the negative path compiles
// against an arbitrary capability without implying a real grant.
func CapSecretReadHint() Capability { return Capability("secret:read-hint") }
