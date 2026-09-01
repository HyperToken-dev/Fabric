package adminauth

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/HyperToken-dev/fabric/internal/config"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/openidConnect"
)

const providerName = "fabric"

// Manager owns OIDC provider registration and claim-to-principal conversion.
//
// Concurrency: Manager is safe for concurrent request use after construction;
// its provider and config values are immutable.
type Manager struct {
	provider goth.Provider
}

// NewManager validates OIDC discovery and registers the configured Goth
// provider. Callers should construct it during startup so configuration errors
// fail fast before the admin server accepts traffic.
func NewManager(db *sql.DB, cfg config.OAuthConfig) (*Manager, error) {
	provider, err := openidConnect.NewNamed(providerName, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL, cfg.IssuerURL, cfg.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("create oidc provider: %w", err)
	}
	goth.UseProviders(provider)
	return &Manager{provider: provider}, nil
}

// Provider returns the configured Goth provider. The value is immutable after
// Manager construction and can be reused by concurrent requests.
func (m *Manager) Provider() goth.Provider {
	return m.provider
}

// ResolvePrincipal maps upstream OIDC claims into the session identity snapshot.
// Casdoor emits its stable user OpenID as `id`; Fabric stores that value as
// owner_openid and leaves login admission plus permission assignment upstream.
func (m *Manager) ResolvePrincipal(oidcUser goth.User) (Principal, error) {
	openid := strings.TrimSpace(claimString(oidcUser.RawData, "id"))
	email := strings.ToLower(strings.TrimSpace(oidcUser.Email))
	if openid == "" {
		return Principal{}, fmt.Errorf("oidc user must include id")
	}
	permissions := []string{}
	if values, ok := oidcUser.RawData["permissions"].([]interface{}); ok {
		for _, value := range values {
			if permission, ok := value.(string); ok && strings.TrimSpace(permission) != "" {
				permissions = append(permissions, strings.TrimSpace(permission))
				continue
			}
			if permission, ok := value.(map[string]interface{}); ok {
				name, _ := permission["name"].(string)
				if name = strings.TrimSpace(name); name != "" {
					permissions = append(permissions, name)
				}
			}
		}
	}
	if values, ok := oidcUser.RawData["permissions"].([]string); ok {
		for _, value := range values {
			if permission := strings.TrimSpace(value); permission != "" {
				permissions = append(permissions, permission)
			}
		}
	}
	role := RoleUser
	for _, permission := range permissions {
		if permission == AdminPermission {
			role = RoleAdmin
			break
		}
	}
	return Principal{
		OpenID:      openid,
		Email:       email,
		DisplayName: oidcUser.Name,
		AvatarURL:   oidcUser.AvatarURL,
		Role:        role,
		Permissions: permissions,
	}, nil
}

// SystemPrincipal returns the local administrator identity used when OAuth is
// disabled. It keeps development mode explicit without recreating local users.
func SystemPrincipal() Principal {
	return Principal{
		OpenID:      "system",
		Email:       "system@fabric.local",
		DisplayName: "Fabric System",
		Role:        RoleAdmin,
		Permissions: []string{AdminPermission},
	}
}

func claimString(claims map[string]interface{}, key string) string {
	value, _ := claims[key].(string)
	return value
}

// SecureCookie reports whether OAuth cookies should use the Secure flag based
// on the configured redirect URL scheme.
func SecureCookie(redirectURL string) bool {
	parsed, err := url.Parse(redirectURL)
	return err == nil && parsed.Scheme == "https"
}
