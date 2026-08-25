package adminauth

import (
	"testing"

	"github.com/markbates/goth"
)

func TestResolvePrincipalRequiresID(t *testing.T) {
	manager := &Manager{}

	if _, err := manager.ResolvePrincipal(goth.User{Email: "user@example.com"}); err == nil {
		t.Fatal("expected missing id to be rejected")
	}
}

func TestResolvePrincipalUsesUpstreamPermissions(t *testing.T) {
	manager := &Manager{}

	principal, err := manager.ResolvePrincipal(goth.User{
		Email:     "ADMIN@example.com",
		Name:      "Admin",
		AvatarURL: "https://example.com/avatar.png",
		RawData: map[string]interface{}{
			"id":          "admin-openid",
			"permissions": []interface{}{"fabric_admin", "other"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if principal.OpenID != "admin-openid" || principal.Email != "admin@example.com" || principal.Role != RoleAdmin {
		t.Fatalf("principal = %+v", principal)
	}
	if len(principal.Permissions) != 2 {
		t.Fatalf("permissions = %+v", principal.Permissions)
	}
}

func TestResolvePrincipalDefaultsToUserWithoutAdminPermission(t *testing.T) {
	manager := &Manager{}

	principal, err := manager.ResolvePrincipal(goth.User{
		Email: "user@example.com",
		RawData: map[string]interface{}{
			"id":          "user-openid",
			"permissions": []interface{}{"fabric:read", 42, ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if principal.Role != RoleUser {
		t.Fatalf("role = %q, want %q", principal.Role, RoleUser)
	}
	if len(principal.Permissions) != 1 || principal.Permissions[0] != "fabric:read" {
		t.Fatalf("permissions = %+v", principal.Permissions)
	}
}

func TestResolvePrincipalTrimsStringPermissions(t *testing.T) {
	manager := &Manager{}

	principal, err := manager.ResolvePrincipal(goth.User{
		Email: "admin@example.com",
		RawData: map[string]interface{}{
			"id":          "admin-openid",
			"permissions": []string{" fabric_admin ", ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if principal.Role != RoleAdmin || len(principal.Permissions) != 1 || principal.Permissions[0] != AdminPermission {
		t.Fatalf("principal = %+v", principal)
	}
}

func TestResolvePrincipalUsesCasdoorPermissionObjects(t *testing.T) {
	manager := &Manager{}

	principal, err := manager.ResolvePrincipal(goth.User{
		Email: "admin@example.com",
		RawData: map[string]interface{}{
			"id": "admin-openid",
			"permissions": []interface{}{
				map[string]interface{}{"name": "fabric_admin", "displayName": "Fabric Admin"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if principal.Role != RoleAdmin || len(principal.Permissions) != 1 || principal.Permissions[0] != AdminPermission {
		t.Fatalf("principal = %+v", principal)
	}
}
