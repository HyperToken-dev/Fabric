package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/repository"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UsageService struct {
	queries  *repository.Queries
	location *time.Location
	now      func() time.Time
}

func NewUsageService(db *sql.DB, location *time.Location) *UsageService {
	if location == nil {
		location = time.UTC
	}
	return &UsageService{queries: repository.New(db), location: location, now: time.Now}
}

func (s *UsageService) GetUsageByKeyID(ctx context.Context, req *proto.GetUsageByKeyIDRequest) (*proto.GetUsageResponse, error) {
	logs, err := s.queries.GetUsageLogsByKeyID(ctx, repository.GetUsageLogsByKeyIDParams{
		KeyID:  req.KeyId,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		zap.L().Error("get usage by key id failed", zap.Error(err), zap.Int32("key_id", req.KeyId))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by key id", zap.Int32("key_id", req.KeyId), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByKeyHash(ctx context.Context, req *proto.GetUsageByKeyHashRequest) (*proto.GetUsageResponse, error) {
	logs, err := s.queries.GetUsageLogsByKeyHash(ctx, repository.GetUsageLogsByKeyHashParams{
		KeyHash: sql.NullString{String: req.KeyHash, Valid: true},
		Limit:   100,
		Offset:  0,
	})
	if err != nil {
		zap.L().Error("get usage by key hash failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by key hash", zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByChannelID(ctx context.Context, req *proto.GetUsageByChannelIDRequest) (*proto.GetUsageResponse, error) {
	logs, err := s.queries.GetUsageLogsByChannel(ctx, repository.GetUsageLogsByChannelParams{
		ChannelID: req.ChannelId,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		zap.L().Error("get usage by channel id failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by channel id", zap.Int32("channel_id", req.ChannelId), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByModelID(ctx context.Context, req *proto.GetUsageByModelIDRequest) (*proto.GetUsageResponse, error) {
	logs, err := s.queries.GetUsageLogsByModelID(ctx, repository.GetUsageLogsByModelIDParams{
		ModelID: req.ModelId,
		Limit:   100,
		Offset:  0,
	})
	if err != nil {
		zap.L().Error("get usage by model id failed", zap.Error(err), zap.Int32("model_id", req.ModelId))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by model id", zap.Int32("model_id", req.ModelId), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByDeadlineAndKeyHash(ctx context.Context, req *proto.GetUsageByDeadlineAndKeyHashRequest) (*proto.GetUsageResponse, error) {
	var deadline time.Time
	if req.Deadline != nil {
		deadline = req.Deadline.AsTime()
	}

	stats, err := s.queries.GetUsageStatsByKeyHash(ctx, repository.GetUsageStatsByKeyHashParams{
		KeyHash: sql.NullString{String: req.KeyHash, Valid: true},
		Column2: deadline,
		Column3: time.Now().UTC(),
	})
	if err != nil {
		zap.L().Error("get usage stats by key hash failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)), zap.Time("deadline", deadline))
		return nil, err
	}

	pbLogs := make([]*proto.UsageLog, 0, len(stats))
	for _, stat := range stats {
		pbLogs = append(pbLogs, &proto.UsageLog{
			ModelId:          stat.ModelID,
			PromptTokens:     fmt.Sprintf("%d", stat.TotalPromptTokens),
			CompletionTokens: fmt.Sprintf("%d", stat.TotalCompletionTokens),
		})
	}
	zap.L().Info("usage stats retrieved by key hash", zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)), zap.Time("deadline", deadline), zap.Int("count", len(pbLogs)))
	return &proto.GetUsageResponse{UsageLog: pbLogs}, nil
}

func (s *UsageService) GetUsageSummary(ctx context.Context, req *proto.GetUsageSummaryRequest) (*proto.GetUsageResponse, error) {
	stats, err := s.queries.GetUsageStatsGlobal(ctx, repository.GetUsageStatsGlobalParams{})
	if err != nil {
		zap.L().Error("get usage summary failed", zap.Error(err))
		return nil, err
	}

	var totalPrompt, totalCompletion int64
	for _, stat := range stats {
		totalPrompt += stat.TotalPromptTokens
		totalCompletion += stat.TotalCompletionTokens
	}

	zap.L().Info("usage summary retrieved", zap.Int("count", len(stats)), zap.Int64("prompt_tokens", totalPrompt), zap.Int64("completion_tokens", totalCompletion))
	return &proto.GetUsageResponse{
		UsageLog: []*proto.UsageLog{{
			PromptTokens:     fmt.Sprintf("%d", totalPrompt),
			CompletionTokens: fmt.Sprintf("%d", totalCompletion),
		}},
	}, nil
}

func (s *UsageService) GetUsageDashboard(ctx context.Context, req *proto.GetUsageDashboardRequest) (*proto.GetUsageDashboardResponse, error) {
	now := s.now().In(s.location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	timelineStart := todayStart.AddDate(0, 0, -6)

	totals, err := s.queries.GetUsageDashboardTotals(ctx, repository.GetUsageDashboardTotalsParams{
		StartAt: todayStart.UTC(),
		EndAt:   tomorrowStart.UTC(),
	})
	if err != nil {
		zap.L().Error("get usage dashboard totals failed", zap.Error(err))
		return nil, err
	}

	rows, err := s.queries.GetUsageDashboardTimeline(ctx, repository.GetUsageDashboardTimelineParams{
		TimeZone: s.location.String(),
		StartAt:  timelineStart.UTC(),
		EndAt:    tomorrowStart.UTC(),
	})
	if err != nil {
		zap.L().Error("get usage dashboard timeline failed", zap.Error(err))
		return nil, err
	}

	byDate := make(map[string]repository.GetUsageDashboardTimelineRow, len(rows))
	for _, row := range rows {
		byDate[row.Date.Format("2006-01-02")] = row
	}

	recentDays := make([]*proto.UsageTimelinePoint, 0, 7)
	for day := timelineStart; day.Before(tomorrowStart); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		row := byDate[date]
		recentDays = append(recentDays, &proto.UsageTimelinePoint{
			Date:             date,
			PromptTokens:     row.TotalPromptTokens,
			CompletionTokens: row.TotalCompletionTokens,
			TotalTokens:      row.TotalPromptTokens + row.TotalCompletionTokens,
			RequestCount:     row.RequestCount,
		})
	}

	return &proto.GetUsageDashboardResponse{
		TimeZone: s.location.String(),
		Today: &proto.UsageTotals{
			PromptTokens:     totals.TotalPromptTokens,
			CompletionTokens: totals.TotalCompletionTokens,
			TotalTokens:      totals.TotalPromptTokens + totals.TotalCompletionTokens,
			RequestCount:     totals.RequestCount,
		},
		RecentDays: recentDays,
	}, nil
}

func repoLogsToProto(logs []repository.UsageLog) []*proto.UsageLog {
	result := make([]*proto.UsageLog, len(logs))
	for i, l := range logs {
		result[i] = &proto.UsageLog{
			UsageId:          l.ID.String(),
			KeyId:            l.KeyID,
			ModelId:          l.ModelID,
			ChannelId:        l.ChannelID,
			PromptTokens:     strconv.FormatInt(l.PromptTokens, 10),
			CompletionTokens: strconv.FormatInt(l.CompletionTokens, 10),
			CreatedAt:        timestamppb.New(l.CreatedAt),
		}
	}
	return result
}
