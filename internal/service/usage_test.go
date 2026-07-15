package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	proto "fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUsageServiceLookupMethods(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db)
	now := time.Now()
	id := uuid.New()
	usageRows := []string{"id", "key_id", "channel_id", "model_id", "prompt_tokens", "completion_tokens", "created_at"}

	mock.ExpectQuery("SELECT id, key_id, channel_id, model_id, prompt_tokens, completion_tokens, created_at FROM usage_logs").
		WithArgs(int32(1), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(4), int64(5), now))
	byKey, err := svc.GetUsageByKeyID(context.Background(), &proto.GetUsageByKeyIDRequest{KeyId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKey.UsageLog) != 1 || byKey.UsageLog[0].UsageId != id.String() || byKey.UsageLog[0].PromptTokens != "4" || byKey.UsageLog[0].CompletionTokens != "5" {
		t.Fatalf("by key usage = %+v", byKey.UsageLog)
	}

	mock.ExpectQuery("JOIN api_keys ON usage_logs.key_id = api_keys.id").
		WithArgs(sql.NullString{String: "hash", Valid: true}, int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(6), int64(7), now))
	byHash, err := svc.GetUsageByKeyHash(context.Background(), &proto.GetUsageByKeyHashRequest{KeyHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byHash.UsageLog) != 1 || byHash.UsageLog[0].PromptTokens != "6" {
		t.Fatalf("by hash usage = %+v", byHash.UsageLog)
	}

	mock.ExpectQuery("WHERE channel_id = \\$1").
		WithArgs(int32(2), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(8), int64(9), now))
	byChannel, err := svc.GetUsageByChannelID(context.Background(), &proto.GetUsageByChannelIDRequest{ChannelId: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(byChannel.UsageLog) != 1 || byChannel.UsageLog[0].PromptTokens != "8" {
		t.Fatalf("by channel usage = %+v", byChannel.UsageLog)
	}

	mock.ExpectQuery("WHERE model_id = \\$1").
		WithArgs(int32(3), int32(100), int32(0)).
		WillReturnRows(sqlmock.NewRows(usageRows).AddRow(id, int32(1), int32(2), int32(3), int64(10), int64(11), now))
	byModel, err := svc.GetUsageByModelID(context.Background(), &proto.GetUsageByModelIDRequest{ModelId: 3})
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
	svc := NewUsageService(db)
	deadline := time.Now().Add(-time.Hour).UTC()

	mock.ExpectQuery("WHERE api_keys.key_hash = \\$1").
		WithArgs(sql.NullString{String: "hash", Valid: true}, deadline, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"model_id", "total_prompt_tokens", "total_completion_tokens", "request_count"}).
			AddRow(int32(3), int64(10), int64(20), int64(2)))
	deadlineResp, err := svc.GetUsageByDeadlineAndKeyHash(context.Background(), &proto.GetUsageByDeadlineAndKeyHashRequest{KeyHash: "hash", Deadline: timestamppb.New(deadline)})
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
	summary, err := svc.GetUsageSummary(context.Background(), &proto.GetUsageSummaryRequest{})
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
	svc := NewUsageService(db)

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(int32(1), int32(100), int32(0)).
		WillReturnError(sql.ErrConnDone)
	if _, err := svc.GetUsageByKeyID(context.Background(), &proto.GetUsageByKeyIDRequest{KeyId: 1}); err == nil {
		t.Fatal("GetUsageByKeyID() error = nil")
	}
}
