package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerRoutesAdminAPI(t *testing.T) {
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proto.ChannelService/ListChannels" {
			t.Fatalf("unexpected admin path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin-api/proto.ChannelService/ListChannels", nil)
	Handler(admin).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerPreservesNativeAdminAPI(t *testing.T) {
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proto.ChannelService/ListChannels" {
			t.Fatalf("unexpected admin path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/proto.ChannelService/ListChannels", nil)
	Handler(admin).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerServesSPA(t *testing.T) {
	admin := http.NotFoundHandler()
	for _, requestPath := range []string{"/", "/channels"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, requestPath, nil)
		Handler(admin).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status for %s: %d", requestPath, recorder.Code)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("unexpected content type for %s: %s", requestPath, contentType)
		}
	}
}
