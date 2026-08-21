package adminauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieManagerSessionRoundTrip(t *testing.T) {
	cookies := NewCookieManager("01234567890123456789012345678901", false)
	recorder := httptest.NewRecorder()
	cookies.SetSession(recorder, 42)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}
	userID, err := cookies.SessionUserID(req)
	if err != nil {
		t.Fatal(err)
	}
	if userID != 42 {
		t.Fatalf("SessionUserID = %d, want 42", userID)
	}
}

func TestCookieManagerRejectsTamperedSession(t *testing.T) {
	cookies := NewCookieManager("01234567890123456789012345678901", false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tampered"})
	if _, err := cookies.SessionUserID(req); err == nil {
		t.Fatal("expected tampered cookie to be rejected")
	}
}
