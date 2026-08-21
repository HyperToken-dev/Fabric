package adminauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/HyperToken-dev/fabric/internal/config"
	"github.com/HyperToken-dev/fabric/internal/repository"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/openidConnect"
)

const providerName = "fabric"

// Manager owns OIDC provider registration and user provisioning policy.
//
// Concurrency: Manager is safe for concurrent request use after construction;
// its maps and config values are immutable and repository calls use sql.DB's
// concurrency-safe connection pool.
type Manager struct {
	cfg            config.OAuthConfig
	queries        *repository.Queries
	provider       goth.Provider
	allowedDomains map[string]struct{}
	adminEmails    map[string]struct{}
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
	m := &Manager{
		cfg:            cfg,
		queries:        repository.New(db),
		provider:       provider,
		allowedDomains: normalizedSet(cfg.AllowedDomains),
		adminEmails:    normalizedSet(cfg.AdminEmails),
	}
	return m, nil
}

// Provider returns the configured Goth provider. The value is immutable after
// Manager construction and can be reused by concurrent requests.
func (m *Manager) Provider() goth.Provider {
	return m.provider
}

// ResolveUser maps a successful OIDC identity to a persisted Fabric user while
// enforcing allowed-domain, auto-provisioning, and disabled-user policy.
func (m *Manager) ResolveUser(ctx context.Context, oidcUser goth.User) (repository.User, error) {
	issuer := claimString(oidcUser.RawData, "iss")
	subject := oidcUser.UserID
	if subject == "" {
		subject = claimString(oidcUser.RawData, "sub")
	}
	email := strings.ToLower(strings.TrimSpace(oidcUser.Email))
	if issuer == "" || subject == "" || email == "" {
		return repository.User{}, fmt.Errorf("oidc user must include issuer, subject, and email")
	}
	if !m.emailAllowed(email) {
		return repository.User{}, fmt.Errorf("email is not allowed")
	}
	role := m.roleForEmail(email)
	arg := repository.GetUserByIssuerSubjectParams{Issuer: issuer, Subject: subject}
	existing, err := m.queries.GetUserByIssuerSubject(ctx, arg)
	if err == nil {
		if existing.Status != "active" {
			return repository.User{}, fmt.Errorf("user is disabled")
		}
		return m.queries.UpdateUserLoginProfile(ctx, repository.UpdateUserLoginProfileParams{
			Issuer:      issuer,
			Subject:     subject,
			Email:       email,
			DisplayName: oidcUser.Name,
			AvatarUrl:   oidcUser.AvatarURL,
			Role:        role,
		})
	}
	if err != sql.ErrNoRows {
		return repository.User{}, err
	}
	if !m.cfg.AutoProvision {
		return repository.User{}, fmt.Errorf("user auto-provisioning is disabled")
	}
	return m.queries.CreateUser(ctx, repository.CreateUserParams{
		Issuer:      issuer,
		Subject:     subject,
		Email:       email,
		DisplayName: oidcUser.Name,
		AvatarUrl:   oidcUser.AvatarURL,
		Role:        role,
	})
}

// SystemUser returns the built-in administrator used only when OAuth is
// disabled. This preserves required API key ownership in unauthenticated local
// development without making browser sessions optional in OAuth deployments.
func SystemUser(ctx context.Context, db *sql.DB) (repository.User, error) {
	user, err := repository.New(db).GetSystemUser(ctx)
	if err != nil {
		return repository.User{}, fmt.Errorf("get system user: %w", err)
	}
	return user, nil
}

func (m *Manager) emailAllowed(email string) bool {
	return EmailAllowed(m.allowedDomains, email)
}

func (m *Manager) roleForEmail(email string) string {
	return RoleForEmail(m.adminEmails, m.cfg.DefaultRole, email)
}

// EmailAllowed applies the configured domain allow-list to a normalized email.
// Empty domains intentionally allow any email so private deployments can rely on
// the upstream OIDC provider as the sole admission boundary.
func EmailAllowed(allowedDomains map[string]struct{}, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(allowedDomains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	_, ok := allowedDomains[email[at+1:]]
	return ok
}

// RoleForEmail assigns the coarse Fabric role from configured admin emails.
// OIDC groups are intentionally ignored so role behavior is deterministic
// across providers that expose different claim sets.
func RoleForEmail(adminEmails map[string]struct{}, defaultRole, email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, ok := adminEmails[email]; ok {
		return RoleAdmin
	}
	role := strings.TrimSpace(defaultRole)
	if role == "" {
		return RoleUser
	}
	return role
}

func normalizedSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
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
