package adminauth

import (
	"context"
	"fmt"

	"github.com/HyperToken-dev/fabric/internal/repository"
)

type contextKey string

const currentUserKey contextKey = "fabric-current-user"

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// WithUser attaches the authenticated Fabric user to a request context.
//
// The returned context is safe to pass across goroutines under normal request
// context rules. The user value is copied so callers cannot mutate shared state.
func WithUser(ctx context.Context, user repository.User) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

// UserFromContext returns the authenticated Fabric user previously attached by
// session middleware. Callers must treat a missing user as an unauthenticated
// request rather than falling back to administrator privileges.
func UserFromContext(ctx context.Context) (repository.User, bool) {
	user, ok := ctx.Value(currentUserKey).(repository.User)
	return user, ok
}

// RequireUser returns the authenticated Fabric user or a stable authorization
// error suitable for service-layer checks.
func RequireUser(ctx context.Context) (repository.User, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return repository.User{}, fmt.Errorf("authenticated user is required")
	}
	return user, nil
}

// RequireAdmin returns the authenticated Fabric administrator or rejects the
// request. This check belongs in services because UI hiding is not a security
// boundary.
func RequireAdmin(ctx context.Context) (repository.User, error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return repository.User{}, err
	}
	if user.Role != RoleAdmin {
		return repository.User{}, fmt.Errorf("admin role is required")
	}
	return user, nil
}
