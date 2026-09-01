package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/google/uuid"
)

func TestClientApiKeyServiceScopesListAndRevokeByOpenID(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewApiKeyService(db)
	now := time.Now()

	mock.ExpectQuery("WHERE api_keys.owner_openid = \\$1").
		WithArgs("user-openid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_hash", "key_name", "channel_id", "created_at", "owner_openid", "channel_name"}).
			AddRow(int32(1), sql.NullString{String: "hash", Valid: true}, "client", int32(7), now, "user-openid", "default"))
	resp, err := svc.ListClientApiKeys(userTestContext(), &proto.ListClientApiKeysRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ApiKeys) != 1 || resp.ApiKeys[0].OwnerOpenid != "user-openid" {
		t.Fatalf("api keys = %+v", resp.ApiKeys)
	}

	mock.ExpectExec("WHERE key_hash = \\$1 AND owner_openid = \\$2").
		WithArgs(sql.NullString{String: "hash", Valid: true}, "user-openid").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := svc.RevokeClientApiKey(userTestContext(), &proto.RevokeClientApiKeyRequest{KeyHash: "hash"}); err != nil {
		t.Fatal(err)
	}
}

func TestUsageServiceScopesUserLookupByOpenID(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery("WHERE key_id = \\$1 AND owner_openid = \\$2").
		WithArgs(int32(9), "user-openid", int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "channel_id", "model_id", "prompt_tokens", "completion_tokens", "owner_openid", "created_at"}).
			AddRow(id, int32(9), int32(2), int32(3), int64(4), int64(5), "user-openid", now))
	resp, err := svc.GetUsageByKeyID(userTestContext(), &proto.GetUsageByKeyIDRequest{KeyId: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.UsageLog) != 1 || resp.UsageLog[0].OwnerOpenid != "user-openid" {
		t.Fatalf("usage logs = %+v", resp.UsageLog)
	}
}

func TestIntegralLogServiceScopesUserLookupByOpenID(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewIntegralLogService(db)
	now := time.Now()

	mock.ExpectQuery("WHERE id = \\$1 AND owner_openid = \\$2").
		WithArgs(int32(5), "user-openid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "context", "response", "key_id", "owner_openid", "created_at"}).
			AddRow(int32(5), []byte(`{"model":"gpt"}`), sql.NullString{String: "ok", Valid: true}, int32(9), "user-openid", now))
	resp, err := svc.GetIntegralLog(userTestContext(), &proto.GetIntegralLogRequest{Id: 5})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Log.OwnerOpenid != "user-openid" {
		t.Fatalf("integral log = %+v", resp.Log)
	}
}

func TestAdminUsageDashboardFiltersByOwnerOpenID(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)
	svc.now = func() time.Time {
		return time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	}

	mock.ExpectQuery("WHERE owner_openid = \\$1").
		WithArgs("owner-openid", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).AddRow(int64(1), int64(2), int64(1)))
	mock.ExpectQuery("WHERE owner_openid = \\$2").
		WithArgs("UTC", "owner-openid", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}))

	resp, err := svc.GetUsageDashboard(adminTestContext(), &proto.GetUsageDashboardRequest{OwnerOpenid: "owner-openid"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Today.TotalTokens != 3 {
		t.Fatalf("dashboard today = %+v", resp.Today)
	}
}
