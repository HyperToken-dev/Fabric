package adminauth

import "testing"

func TestRoleForEmailUsesAdminEmailsOnly(t *testing.T) {
	admins := map[string]struct{}{"admin@example.com": {}}
	if got := RoleForEmail(admins, RoleUser, "ADMIN@example.com"); got != RoleAdmin {
		t.Fatalf("RoleForEmail admin = %q, want %q", got, RoleAdmin)
	}
	if got := RoleForEmail(admins, RoleUser, "user@example.com"); got != RoleUser {
		t.Fatalf("RoleForEmail user = %q, want %q", got, RoleUser)
	}
}

func TestEmailAllowed(t *testing.T) {
	if !EmailAllowed(nil, "anywhere@example.net") {
		t.Fatal("empty allow-list should allow provider-authenticated emails")
	}
	allowed := map[string]struct{}{"example.com": {}}
	if !EmailAllowed(allowed, "user@example.com") {
		t.Fatal("expected example.com email to be allowed")
	}
	if EmailAllowed(allowed, "user@other.test") {
		t.Fatal("expected non-allowed domain to be rejected")
	}
}
