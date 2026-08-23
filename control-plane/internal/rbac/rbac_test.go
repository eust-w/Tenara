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
