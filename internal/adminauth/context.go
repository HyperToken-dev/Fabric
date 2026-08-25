package adminauth

import (
	"context"
	"fmt"
)

type contextKey string

const currentUserKey contextKey = "fabric-current-user"

const (
	AdminPermission = "fabric_admin"
	RoleAdmin       = "admin"
	RoleUser        = "user"
)

// Principal is the upstream OIDC identity snapshot trusted by management APIs.
//
// Concurrency: Principal values are copied into request contexts and are safe to
// read across request goroutines. Permissions must be treated as immutable after
// construction because authorization decisions depend on a stable login snapshot.
type Principal struct {
	OpenID      string
	Email       string
	DisplayName string
	AvatarURL   string
	Role        string
	Permissions []string
}

// WithPrincipal attaches the authenticated OIDC principal to a request context.
//
// The returned context is safe to pass across goroutines under normal request
// context rules. The principal value is copied so callers cannot mutate shared
// state outside the permissions slice they already own.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, currentUserKey, principal)
}

// PrincipalFromContext returns the authenticated OIDC principal attached by
// session middleware. Missing principals are unauthenticated requests.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(currentUserKey).(Principal)
	return principal, ok
}

// RequireUser returns the authenticated OIDC principal or a stable authorization
// error suitable for service-layer checks.
func RequireUser(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, fmt.Errorf("authenticated user is required")
	}
	return principal, nil
}

// RequireAdmin returns an authenticated principal carrying the upstream admin
// permission. This check belongs in services because UI hiding is not a security
// boundary.
func RequireAdmin(ctx context.Context) (Principal, error) {
	principal, err := RequireUser(ctx)
	if err != nil {
		return Principal{}, err
	}
	for _, permission := range principal.Permissions {
		if permission == AdminPermission {
			return principal, nil
		}
	}
	return Principal{}, fmt.Errorf("admin permission is required")
}
