package adminauth

import (
	"context"
	"testing"
)

func TestRequireAdminRejectsNonAdminPermission(t *testing.T) {
	ctx := WithPrincipal(context.Background(), Principal{
		OpenID:      "user-openid",
		Role:        RoleAdmin,
		Permissions: []string{"fabric:read"},
	})

	if _, err := RequireAdmin(ctx); err == nil {
		t.Fatal("expected non-admin permission to be rejected")
	}
}
