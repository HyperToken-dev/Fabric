package adminauth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/repository"
	"go.uber.org/zap"
)

// Handler serves OIDC browser routes and session protection for the admin
// server. It is safe for concurrent use after construction.
type Handler struct {
	manager *Manager
	cookies *CookieManager
	queries *repository.Queries
}

// NewHandler creates admin-server authentication handlers. The manager and
// cookie manager must be non-nil when OAuth is enabled.
func NewHandler(db *sql.DB, manager *Manager, cookies *CookieManager) *Handler {
	return &Handler{manager: manager, cookies: cookies, queries: repository.New(db)}
}

// Login starts the OIDC authorization code flow and stores the short-lived Goth
// session payload in a signed cookie until callback.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := NewState()
	if err != nil {
		http.Error(w, "create oauth state", http.StatusInternalServerError)
		return
	}
	session, err := h.manager.Provider().BeginAuth(state)
	if err != nil {
		http.Error(w, "begin oauth", http.StatusInternalServerError)
		return
	}
	encodedSession := base64.RawURLEncoding.EncodeToString([]byte(session.Marshal()))
	h.cookies.SetOAuthSession(w, state+":"+encodedSession)
	authURL, err := session.GetAuthURL()
	if err != nil {
		http.Error(w, "build oauth url", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback completes the OIDC flow, persists or updates the Fabric user, and
// writes the long-lived browser session cookie.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	stored, err := h.cookies.OAuthSession(r)
	if err != nil {
		http.Error(w, "missing oauth session", http.StatusUnauthorized)
		return
	}
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[0] != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusUnauthorized)
		return
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		http.Error(w, "invalid oauth session", http.StatusUnauthorized)
		return
	}
	session, err := h.manager.Provider().UnmarshalSession(string(decoded))
	if err != nil {
		http.Error(w, "restore oauth session", http.StatusUnauthorized)
		return
	}
	if _, err := session.Authorize(h.manager.Provider(), r.URL.Query()); err != nil {
		http.Error(w, "authorize oauth session", http.StatusUnauthorized)
		return
	}
	gothUser, err := h.manager.Provider().FetchUser(session)
	if err != nil {
		http.Error(w, "fetch oauth user", http.StatusUnauthorized)
		return
	}
	user, err := h.manager.ResolveUser(r.Context(), gothUser)
	if err != nil {
		zap.L().Warn("oidc login rejected", zap.Error(err), zap.String("email", gothUser.Email))
		http.Error(w, "login rejected", http.StatusForbidden)
		return
	}
	h.cookies.ClearOAuthSession(w)
	h.cookies.SetSession(w, user.ID)
	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout clears the browser session and returns an empty Connect response.
func (h *Handler) Logout(ctx context.Context, req *connect.Request[proto.LogoutRequest]) (*connect.Response[proto.LogoutResponse], error) {
	resp := connect.NewResponse(&proto.LogoutResponse{})
	if h.cookies != nil {
		h.cookies.ClearSessionHeader(resp.Header())
	}
	return resp, nil
}

// GetCurrentUser returns the authenticated user stored in request context.
func (h *Handler) GetCurrentUser(ctx context.Context, req *connect.Request[proto.GetCurrentUserRequest]) (*connect.Response[proto.GetCurrentUserResponse], error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return connect.NewResponse(&proto.GetCurrentUserResponse{User: UserToProto(user)}), nil
}

// Middleware protects the admin server and injects the authenticated user into
// request context. OAuth routes and static login assets are intentionally left
// public so unauthenticated users can start login.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/auth/login") || strings.HasPrefix(r.URL.Path, "/auth/callback") {
			next.ServeHTTP(w, r)
			return
		}
		userID, err := h.cookies.SessionUserID(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/admin-api") || strings.HasPrefix(r.URL.Path, "/proto.") {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		user, err := h.queries.GetUserByID(r.Context(), userID)
		if err != nil || user.Status != "active" {
			if err != nil {
				zap.L().Warn("session user lookup failed", zap.Error(err), zap.Int32("user_id", userID))
			}
			h.cookies.ClearSession(w)
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

// UserToProto converts a repository user to the public current-user message.
func UserToProto(user repository.User) *proto.CurrentUser {
	return &proto.CurrentUser{
		UserId:      user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarUrl:   user.AvatarUrl,
		Role:        user.Role,
	}
}
