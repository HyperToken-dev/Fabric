package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/DATA-DOG/go-sqlmock"
	gen "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
	"github.com/HyperToken-dev/fabric/internal/repository"
	"github.com/HyperToken-dev/fabric/internal/service"
)

func TestUsageHandlersDelegateToService(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := NewServer(nil, nil, nil, service.NewUsageService(db, time.UTC), nil, nil)

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(int32(7), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "channel_id", "model_id", "prompt_tokens", "completion_tokens", "created_at"}))
	resp, err := srv.GetUsageByKeyID(adminServerTestContext(), connect.NewRequest(&gen.GetUsageByKeyIDRequest{KeyId: 7}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.UsageLog == nil {
		t.Fatal("UsageLog is nil")
	}
}

func TestUsageDashboardHandlerIsImplemented(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := NewServer(nil, nil, nil, service.NewUsageService(db, time.UTC), nil, nil)

	mock.ExpectQuery("COALESCE\\(SUM\\(prompt_tokens\\), 0\\)").
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).AddRow(0, 0, 0))
	mock.ExpectQuery("DATE\\(created_at AT TIME ZONE").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}))
	resp, err := srv.GetUsageDashboard(adminServerTestContext(), connect.NewRequest(&gen.GetUsageDashboardRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.TimeZone != "UTC" || len(resp.Msg.RecentDays) != 7 {
		t.Fatalf("dashboard response = %+v", resp.Msg)
	}
}

func TestUsageHandlerMapsServiceErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := NewServer(nil, nil, nil, service.NewUsageService(db, time.UTC), nil, nil)

	mock.ExpectQuery("FROM usage_logs").WillReturnError(context.DeadlineExceeded)
	_, err = srv.GetUsageByKeyID(adminServerTestContext(), connect.NewRequest(&gen.GetUsageByKeyIDRequest{KeyId: 7}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("error code = %v, want internal", connect.CodeOf(err))
	}
}

func adminServerTestContext() context.Context {
	return adminauth.WithUser(context.Background(), repository.User{ID: 1, Role: adminauth.RoleAdmin, Status: "active"})
}
