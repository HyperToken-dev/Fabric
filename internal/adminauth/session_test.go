package adminauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieManagerSessionRoundTrip(t *testing.T) {
	cookies := NewCookieManager("01234567890123456789012345678901", false)
	recorder := httptest.NewRecorder()
	if err := cookies.SetSession(recorder, Principal{
		OpenID:      "admin-openid",
		Email:       "admin@example.com",
		Role:        RoleAdmin,
		Permissions: []string{AdminPermission},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}
	principal, err := cookies.SessionPrincipal(req)
	if err != nil {
		t.Fatal(err)
	}
	if principal.OpenID != "admin-openid" || principal.Role != RoleAdmin || len(principal.Permissions) != 1 {
		t.Fatalf("SessionPrincipal = %+v", principal)
	}
}

func TestCookieManagerRejectsTamperedSession(t *testing.T) {
	cookies := NewCookieManager("01234567890123456789012345678901", false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tampered"})
	if _, err := cookies.SessionPrincipal(req); err == nil {
		t.Fatal("expected tampered cookie to be rejected")
	}
}
