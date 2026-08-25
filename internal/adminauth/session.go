package adminauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "fabric_session"
	oauthCookieName   = "fabric_oauth"
	sessionTTL        = 24 * time.Hour
	oauthTTL          = 10 * time.Minute
)

// CookieManager creates and verifies signed cookies used by the admin server.
//
// Concurrency: CookieManager is safe for concurrent use after construction
// because it owns immutable secret bytes and performs no shared mutation.
type CookieManager struct {
	secret []byte
	secure bool
}

// NewCookieManager creates a cookie signer. The secret must already be checked
// by configuration validation; keeping this constructor small avoids divergent
// validation rules.
func NewCookieManager(secret string, secure bool) *CookieManager {
	return &CookieManager{secret: []byte(secret), secure: secure}
}

// SetSession writes a signed browser session cookie for the OIDC principal.
func (m *CookieManager) SetSession(w http.ResponseWriter, principal Principal) error {
	payload, err := json.Marshal(principal)
	if err != nil {
		return fmt.Errorf("marshal principal session: %w", err)
	}
	m.setSigned(w, sessionCookieName, base64.RawURLEncoding.EncodeToString(payload), sessionTTL)
	return nil
}

// SessionPrincipal verifies the browser session cookie and returns its OIDC
// identity snapshot. The snapshot is authorization state until the next login.
func (m *CookieManager) SessionPrincipal(r *http.Request) (Principal, error) {
	value, err := m.signedValue(r, sessionCookieName)
	if err != nil {
		return Principal{}, err
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Principal{}, fmt.Errorf("decode principal session: %w", err)
	}
	var principal Principal
	if err := json.Unmarshal(payload, &principal); err != nil {
		return Principal{}, fmt.Errorf("unmarshal principal session: %w", err)
	}
	if strings.TrimSpace(principal.OpenID) == "" {
		return Principal{}, fmt.Errorf("session openid is required")
	}
	return principal, nil
}

// ClearSession removes the browser session cookie.
func (m *CookieManager) ClearSession(w http.ResponseWriter) {
	m.clear(w, sessionCookieName)
}

// ClearSessionHeader appends the expired session cookie to a Connect response
// header where no http.ResponseWriter is available.
func (m *CookieManager) ClearSessionHeader(header http.Header) {
	header.Add("Set-Cookie", m.expiredCookie(sessionCookieName).String())
}

// SetOAuthSession stores the Goth session payload and state between OIDC login
// start and callback. The payload is short-lived because it contains OAuth flow
// state, not long-term Fabric authorization.
func (m *CookieManager) SetOAuthSession(w http.ResponseWriter, payload string) {
	m.setSigned(w, oauthCookieName, payload, oauthTTL)
}

// OAuthSession verifies and returns the pending Goth session payload.
func (m *CookieManager) OAuthSession(r *http.Request) (string, error) {
	return m.signedValue(r, oauthCookieName)
}

// ClearOAuthSession removes the short-lived OIDC flow cookie.
func (m *CookieManager) ClearOAuthSession(w http.ResponseWriter) {
	m.clear(w, oauthCookieName)
}

func (m *CookieManager) setSigned(w http.ResponseWriter, name, value string, ttl time.Duration) {
	expires := time.Now().UTC().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%d", value, expires)
	sig := m.signature(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig)),
		Path:     "/",
		Expires:  time.Unix(expires, 0),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *CookieManager) signedValue(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", fmt.Errorf("decode cookie: %w", err)
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid signed cookie format")
	}
	payload := parts[0] + "|" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(m.signature(payload))) {
		return "", fmt.Errorf("invalid signed cookie signature")
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid signed cookie expiry: %w", err)
	}
	if time.Now().UTC().After(time.Unix(expires, 0)) {
		return "", fmt.Errorf("signed cookie expired")
	}
	return parts[0], nil
}

func (m *CookieManager) signature(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *CookieManager) clear(w http.ResponseWriter, name string) {
	http.SetCookie(w, m.expiredCookie(name))
}

func (m *CookieManager) expiredCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// NewState returns an unpredictable OAuth state value for CSRF protection in
// the browser redirect flow.
func NewState() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
