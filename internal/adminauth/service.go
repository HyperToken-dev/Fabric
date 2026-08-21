package adminauth

import (
	"context"

	"connectrpc.com/connect"
	proto "github.com/HyperToken-dev/fabric/gen"
	protoconnect "github.com/HyperToken-dev/fabric/gen/protoconnect"
)

// Service exposes session state over Connect. It is intentionally small so the
// same current-user endpoint works for both OAuth-enabled and local system-user
// modes.
type Service struct {
	protoconnect.UnimplementedAuthServiceHandler
	cookies      *CookieManager
	oauthEnabled bool
}

// NewService creates an AuthService handler. A nil cookie manager is valid for
// OAuth-disabled local mode, where logout becomes a no-op.
func NewService(cookies *CookieManager, oauthEnabled bool) *Service {
	return &Service{cookies: cookies, oauthEnabled: oauthEnabled}
}

// GetCurrentUser returns the authenticated user from request context.
func (s *Service) GetCurrentUser(ctx context.Context, req *connect.Request[proto.GetCurrentUserRequest]) (*connect.Response[proto.GetCurrentUserResponse], error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return connect.NewResponse(&proto.GetCurrentUserResponse{User: &proto.CurrentUser{
		UserId:       user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		AvatarUrl:    user.AvatarUrl,
		Role:         user.Role,
		OauthEnabled: s.oauthEnabled,
	}}), nil
}

// Logout clears the browser session when OAuth cookies are active.
func (s *Service) Logout(ctx context.Context, req *connect.Request[proto.LogoutRequest]) (*connect.Response[proto.LogoutResponse], error) {
	resp := connect.NewResponse(&proto.LogoutResponse{})
	if s.cookies != nil {
		s.cookies.ClearSessionHeader(resp.Header())
	}
	return resp, nil
}
