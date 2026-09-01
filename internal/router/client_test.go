package router

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HyperToken-dev/fabric/internal/auth"
	"github.com/HyperToken-dev/fabric/internal/models"
	"github.com/HyperToken-dev/fabric/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestRouterAuthMiddlewareRejectsMissingOrInvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, cleanup := newRouterMock(t)
	defer cleanup()
	rt := New(repository.New(db), map[int32]Proxy{models.APIFormatOpenAI: &fakeProxy{}})
	engine := rt.RegisterProxyRoutes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d, want 401", rec.Code)
	}

	for _, header := range []string{"Basic key", "Bearer ", "Bearer   "} {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", header)
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("header %q status = %d, want 401", header, rec.Code)
		}
	}

	mock.ExpectQuery("FROM api_keys").
		WithArgs(sql.NullString{String: auth.HashKey("bad-key"), Valid: true}).
		WillReturnError(sql.ErrNoRows)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer bad-key")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid key status = %d, want 401", rec.Code)
	}
}

func TestRouterAuthMiddlewareRejectsDisabledChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, cleanup := newRouterMock(t)
	defer cleanup()
	rt := New(repository.New(db), map[int32]Proxy{models.APIFormatOpenAI: &fakeProxy{}})
	engine := rt.RegisterProxyRoutes()

	mock.ExpectQuery("FROM api_keys").
		WithArgs(sql.NullString{String: auth.HashKey("key"), Valid: true}).
		WillReturnRows(apiKeyWithChannelRows().AddRow(int32(1), int32(2), "https://upstream", "provider", int32(models.APIFormatOpenAI), int16(2), "owner-openid"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer key")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRouterProxyHandlerDispatchesOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, cleanup := newRouterMock(t)
	defer cleanup()
	proxy := &fakeProxy{}
	rt := New(repository.New(db), map[int32]Proxy{models.APIFormatOpenAI: proxy})
	engine := rt.RegisterProxyRoutes()

	mock.ExpectQuery("FROM api_keys").
		WithArgs(sql.NullString{String: auth.HashKey("key"), Valid: true}).
		WillReturnRows(apiKeyWithChannelRows().AddRow(int32(1), int32(2), "https://upstream", "provider", int32(models.APIFormatOpenAI), int16(channelStatusEnabled), "owner-openid"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer key")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if !proxy.called || proxy.keyID != 1 || proxy.channelID != 2 || proxy.baseURL != "https://upstream" || proxy.providerKey != "provider" {
		t.Fatalf("proxy call = %+v", proxy)
	}
}

func TestRouterProxyHandlerRejectsUnsupportedAPIFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, cleanup := newRouterMock(t)
	defer cleanup()
	rt := New(repository.New(db), map[int32]Proxy{models.APIFormatOpenAI: &fakeProxy{}})
	engine := rt.RegisterProxyRoutes()

	mock.ExpectQuery("FROM api_keys").
		WithArgs(sql.NullString{String: auth.HashKey("key"), Valid: true}).
		WillReturnRows(apiKeyWithChannelRows().AddRow(int32(1), int32(2), "https://upstream", "provider", int32(99), int16(channelStatusEnabled), "owner-openid"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer key")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func apiKeyWithChannelRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"key_id", "channel_id", "base_url", "provider_key", "channel_api_format", "channel_status", "owner_openid"})
}

func newRouterMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	return db, mock, func() { _ = db.Close() }
}

type fakeProxy struct {
	called      bool
	keyID       int32
	channelID   int32
	baseURL     string
	providerKey string
}

func (p *fakeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	p.called = true
	p.keyID = keyID
	p.channelID = channelID
	p.baseURL = baseURL
	p.providerKey = providerKey
	w.WriteHeader(http.StatusAccepted)
}
