package service

import (
	"database/sql"
	"testing"
	"time"

	proto "github.com/HyperToken-dev/fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUsageServiceLookupMethods(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)
	now := time.Now()
	id := uuid.New()
	usageRows := []string{"id", "key_id", "channel_id", "model_id", "prompt_tokens", "completion_tokens", "created_at"}

	mock.ExpectQuery("SELECT id, key_id, channel_id, model_id, prompt_tokens, completion_tokens, created_at FROM usage_logs").
		WithArgs(int32(1), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(4), int64(5), now))
	byKey, err := svc.GetUsageByKeyID(adminTestContext(), &proto.GetUsageByKeyIDRequest{KeyId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKey.UsageLog) != 1 || byKey.UsageLog[0].UsageId != id.String() || byKey.UsageLog[0].PromptTokens != "4" || byKey.UsageLog[0].CompletionTokens != "5" {
		t.Fatalf("by key usage = %+v", byKey.UsageLog)
	}

	mock.ExpectQuery("JOIN api_keys ON usage_logs.key_id = api_keys.id").
		WithArgs(sql.NullString{String: "hash", Valid: true}, int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(6), int64(7), now))
	byHash, err := svc.GetUsageByKeyHash(adminTestContext(), &proto.GetUsageByKeyHashRequest{KeyHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byHash.UsageLog) != 1 || byHash.UsageLog[0].PromptTokens != "6" {
		t.Fatalf("by hash usage = %+v", byHash.UsageLog)
	}

	mock.ExpectQuery("WHERE channel_id = \\$1").
		WithArgs(int32(2), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(8), int64(9), now))
	byChannel, err := svc.GetUsageByChannelID(adminTestContext(), &proto.GetUsageByChannelIDRequest{ChannelId: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(byChannel.UsageLog) != 1 || byChannel.UsageLog[0].PromptTokens != "8" {
		t.Fatalf("by channel usage = %+v", byChannel.UsageLog)
	}

	mock.ExpectQuery("WHERE model_id = \\$1").
		WithArgs(int32(3), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(10), int64(11), now))
	byModel, err := svc.GetUsageByModelID(adminTestContext(), &proto.GetUsageByModelIDRequest{ModelId: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(byModel.UsageLog) != 1 || byModel.UsageLog[0].ModelId != 3 || byModel.UsageLog[0].PromptTokens != "10" {
		t.Fatalf("by model usage = %+v", byModel.UsageLog)
	}
}

func TestUsageServiceSummaries(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)
	deadline := time.Now().Add(-time.Hour).UTC()

	mock.ExpectQuery("WHERE api_keys.key_hash = \\$1").
		WithArgs(sql.NullString{String: "hash", Valid: true}, deadline, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"model_id", "total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(int32(3), int64(10), int64(20), int64(2)))
	deadlineResp, err := svc.GetUsageByDeadlineAndKeyHash(adminTestContext(), &proto.GetUsageByDeadlineAndKeyHashRequest{KeyHash: "hash", Deadline: timestamppb.New(deadline)})
	if err != nil {
		t.Fatal(err)
	}
	if len(deadlineResp.UsageLog) != 1 || deadlineResp.UsageLog[0].ModelId != 3 || deadlineResp.UsageLog[0].PromptTokens != "10" {
		t.Fatalf("deadline usage = %+v", deadlineResp.UsageLog)
	}

	mock.ExpectQuery("GROUP BY channel_id, model_id").
		WillReturnRows(sqlmock.NewRows([]string{"channel_id", "model_id", "total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(int32(1), int32(2), int64(5), int64(7), int64(1)).
			AddRow(int32(1), int32(3), int64(11), int64(13), int64(1)))
	summary, err := svc.GetUsageSummary(adminTestContext(), &proto.GetUsageSummaryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.UsageLog) != 1 || summary.UsageLog[0].PromptTokens != "16" || summary.UsageLog[0].CompletionTokens != "20" {
		t.Fatalf("summary = %+v", summary.UsageLog)
	}
}

func TestUsageServicePropagatesErrors(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(int32(1), int32(100), int32(0)).
		WillReturnError(sql.ErrConnDone)
	if _, err := svc.GetUsageByKeyID(adminTestContext(), &proto.GetUsageByKeyIDRequest{KeyId: 1}); err == nil {
		t.Fatal("GetUsageByKeyID() error = nil")
	}
}

func TestGetUsageDashboardUsesConfiguredCalendarDay(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewUsageService(db, location)
	svc.now = func() time.Time {
		return time.Date(2026, time.July, 20, 8, 30, 0, 0, time.UTC)
	}

	todayStart := time.Date(2026, time.July, 19, 16, 0, 0, 0, time.UTC)
	tomorrowStart := time.Date(2026, time.July, 20, 16, 0, 0, 0, time.UTC)
	timelineStart := time.Date(2026, time.July, 13, 16, 0, 0, 0, time.UTC)
	mock.ExpectQuery("COALESCE\\(SUM\\(prompt_tokens\\), 0\\)").
		WithArgs(todayStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(int64(30), int64(12), int64(4)))
	mock.ExpectQuery("DATE\\(created_at AT TIME ZONE").
		WithArgs("Asia/Shanghai", timelineStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), int64(5), int64(2), int64(1)).
			AddRow(time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC), int64(30), int64(12), int64(4)))

	resp, err := svc.GetUsageDashboard(adminTestContext(), &proto.GetUsageDashboardRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TimeZone != "Asia/Shanghai" {
		t.Fatalf("TimeZone = %q", resp.TimeZone)
	}
	if resp.Today.PromptTokens != 30 || resp.Today.CompletionTokens != 12 || resp.Today.TotalTokens != 42 || resp.Today.RequestCount != 4 {
		t.Fatalf("Today = %+v", resp.Today)
	}
	if len(resp.RecentDays) != 7 {
		t.Fatalf("RecentDays length = %d, want 7", len(resp.RecentDays))
	}
	if resp.RecentDays[0].Date != "2026-07-14" || resp.RecentDays[6].Date != "2026-07-20" {
		t.Fatalf("RecentDays range = %q to %q", resp.RecentDays[0].Date, resp.RecentDays[6].Date)
	}
	if resp.RecentDays[1].TotalTokens != 7 || resp.RecentDays[1].RequestCount != 1 {
		t.Fatalf("July 15 point = %+v", resp.RecentDays[1])
	}
	if resp.RecentDays[2].TotalTokens != 0 || resp.RecentDays[2].RequestCount != 0 {
		t.Fatalf("zero-filled point = %+v", resp.RecentDays[2])
	}
}

func TestGetUsageDashboardDefaultsToUTCAndReturnsEmptyDays(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, nil)
	svc.now = func() time.Time {
		return time.Date(2026, time.July, 20, 23, 0, 0, 0, time.UTC)
	}

	todayStart := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	tomorrowStart := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	timelineStart := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("COALESCE\\(SUM\\(prompt_tokens\\), 0\\)").
		WithArgs(todayStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(int64(0), int64(0), int64(0)))
	mock.ExpectQuery("DATE\\(created_at AT TIME ZONE").
		WithArgs("UTC", timelineStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}))

	resp, err := svc.GetUsageDashboard(adminTestContext(), &proto.GetUsageDashboardRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TimeZone != "UTC" || resp.Today.TotalTokens != 0 || len(resp.RecentDays) != 7 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestGetUsageDashboardHandlesDaylightSavingBoundary(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewUsageService(db, location)
	svc.now = func() time.Time {
		return time.Date(2026, time.March, 8, 16, 0, 0, 0, time.UTC)
	}

	todayStart := time.Date(2026, time.March, 8, 5, 0, 0, 0, time.UTC)
	tomorrowStart := time.Date(2026, time.March, 9, 4, 0, 0, 0, time.UTC)
	timelineStart := time.Date(2026, time.March, 2, 5, 0, 0, 0, time.UTC)
	mock.ExpectQuery("COALESCE\\(SUM\\(prompt_tokens\\), 0\\)").
		WithArgs(todayStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).AddRow(0, 0, 0))
	mock.ExpectQuery("DATE\\(created_at AT TIME ZONE").
		WithArgs("America/New_York", timelineStart, tomorrowStart).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}))

	if _, err := svc.GetUsageDashboard(adminTestContext(), &proto.GetUsageDashboardRequest{}); err != nil {
		t.Fatal(err)
	}
	if tomorrowStart.Sub(todayStart) != 23*time.Hour {
		t.Fatalf("DST day duration = %v, want 23h", tomorrowStart.Sub(todayStart))
	}
}
