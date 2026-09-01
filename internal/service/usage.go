package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
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
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	var logs []repository.UsageLog
	if user.Role == adminauth.RoleAdmin {
		logs, err = s.queries.GetUsageLogsByKeyID(ctx, repository.GetUsageLogsByKeyIDParams{
			KeyID:  req.KeyId,
			Limit:  100,
			Offset: 0,
		})
	} else {
		logs, err = s.queries.GetUsageLogsByKeyIDAndOwnerOpenID(ctx, repository.GetUsageLogsByKeyIDAndOwnerOpenIDParams{
			KeyID:       req.KeyId,
			OwnerOpenid: user.OpenID,
			Limit:       100,
			Offset:      0,
		})
	}
	if err != nil {
		zap.L().Error("get usage by key id failed", zap.Error(err), zap.Int32("key_id", req.KeyId))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by key id", zap.Int32("key_id", req.KeyId), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByKeyHash(ctx context.Context, req *proto.GetUsageByKeyHashRequest) (*proto.GetUsageResponse, error) {
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	var logs []repository.UsageLog
	if user.Role == adminauth.RoleAdmin {
		logs, err = s.queries.GetUsageLogsByKeyHash(ctx, repository.GetUsageLogsByKeyHashParams{
			KeyHash: sql.NullString{String: req.KeyHash, Valid: true},
			Limit:   100,
			Offset:  0,
		})
	} else {
		logs, err = s.queries.GetUsageLogsByKeyHashAndOwnerOpenID(ctx, repository.GetUsageLogsByKeyHashAndOwnerOpenIDParams{
			KeyHash:     sql.NullString{String: req.KeyHash, Valid: true},
			OwnerOpenid: user.OpenID,
			Limit:       100,
			Offset:      0,
		})
	}
	if err != nil {
		zap.L().Error("get usage by key hash failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
		return nil, err
	}
	zap.L().Info("usage logs retrieved by key hash", zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)), zap.Int("count", len(logs)))
	return &proto.GetUsageResponse{UsageLog: repoLogsToProto(logs)}, nil
}

func (s *UsageService) GetUsageByChannelID(ctx context.Context, req *proto.GetUsageByChannelIDRequest) (*proto.GetUsageResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	var totalPrompt, totalCompletion int64
	if user.Role == adminauth.RoleAdmin {
		stats, err := s.queries.GetUsageStatsGlobal(ctx, repository.GetUsageStatsGlobalParams{})
		if err != nil {
			zap.L().Error("get usage summary failed", zap.Error(err))
			return nil, err
		}
		for _, stat := range stats {
			totalPrompt += stat.TotalPromptTokens
			totalCompletion += stat.TotalCompletionTokens
		}
		zap.L().Info("usage summary retrieved", zap.Int("count", len(stats)), zap.Int64("prompt_tokens", totalPrompt), zap.Int64("completion_tokens", totalCompletion))
	} else {
		stats, err := s.queries.GetUsageStatsByOwnerOpenID(ctx, repository.GetUsageStatsByOwnerOpenIDParams{OwnerOpenid: user.OpenID})
		if err != nil {
			zap.L().Error("get owner usage summary failed", zap.Error(err), zap.String("owner_openid", user.OpenID))
			return nil, err
		}
		for _, stat := range stats {
			totalPrompt += stat.TotalPromptTokens
			totalCompletion += stat.TotalCompletionTokens
		}
		zap.L().Info("owner usage summary retrieved", zap.String("owner_openid", user.OpenID), zap.Int("count", len(stats)), zap.Int64("prompt_tokens", totalPrompt), zap.Int64("completion_tokens", totalCompletion))
	}

	return &proto.GetUsageResponse{
		UsageLog: []*proto.UsageLog{{
			PromptTokens:     fmt.Sprintf("%d", totalPrompt),
			CompletionTokens: fmt.Sprintf("%d", totalCompletion),
		}},
	}, nil
}

func (s *UsageService) GetUsageDashboard(ctx context.Context, req *proto.GetUsageDashboardRequest) (*proto.GetUsageDashboardResponse, error) {
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now().In(s.location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	timelineStart := todayStart.AddDate(0, 0, -6)

	var totals dashboardTotalsRow
	var rows []dashboardTimelineRow
	if user.Role == adminauth.RoleAdmin && req.OwnerOpenid == "" {
		dbTotals, err := s.queries.GetUsageDashboardTotals(ctx, repository.GetUsageDashboardTotalsParams{StartAt: todayStart.UTC(), EndAt: tomorrowStart.UTC()})
		if err != nil {
			zap.L().Error("get usage dashboard totals failed", zap.Error(err), zap.String("owner_openid", req.OwnerOpenid), zap.String("role", user.Role))
			return nil, err
		}
		dbRows, err := s.queries.GetUsageDashboardTimeline(ctx, repository.GetUsageDashboardTimelineParams{TimeZone: s.location.String(), StartAt: timelineStart.UTC(), EndAt: tomorrowStart.UTC()})
		if err != nil {
			zap.L().Error("get usage dashboard timeline failed", zap.Error(err), zap.String("owner_openid", req.OwnerOpenid), zap.String("role", user.Role))
			return nil, err
		}
		totals = dashboardTotalsRow(dbTotals)
		rows = convertGlobalTimeline(dbRows)
	} else {
		ownerOpenID := user.OpenID
		if user.Role == adminauth.RoleAdmin && req.OwnerOpenid != "" {
			ownerOpenID = req.OwnerOpenid
		}
		dbTotals, err := s.queries.GetUsageDashboardTotalsByOwnerOpenID(ctx, repository.GetUsageDashboardTotalsByOwnerOpenIDParams{OwnerOpenid: ownerOpenID, StartAt: todayStart.UTC(), EndAt: tomorrowStart.UTC()})
		if err != nil {
			zap.L().Error("get usage dashboard owner totals failed", zap.Error(err), zap.String("owner_openid", ownerOpenID), zap.String("role", user.Role))
			return nil, err
		}
		dbRows, err := s.queries.GetUsageDashboardTimelineByOwnerOpenID(ctx, repository.GetUsageDashboardTimelineByOwnerOpenIDParams{TimeZone: s.location.String(), OwnerOpenid: ownerOpenID, StartAt: timelineStart.UTC(), EndAt: tomorrowStart.UTC()})
		if err != nil {
			zap.L().Error("get usage dashboard owner timeline failed", zap.Error(err), zap.String("owner_openid", ownerOpenID), zap.String("role", user.Role))
			return nil, err
		}
		totals = dashboardTotalsRow(dbTotals)
		rows = convertOwnerTimeline(dbRows)
	}

	byDate := make(map[string]dashboardTimelineRow, len(rows))
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

type dashboardTotalsRow struct {
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	RequestCount          int64
}

type dashboardTimelineRow struct {
	Date                  time.Time
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	RequestCount          int64
}

func convertGlobalTimeline(rows []repository.GetUsageDashboardTimelineRow) []dashboardTimelineRow {
	result := make([]dashboardTimelineRow, len(rows))
	for i, row := range rows {
		result[i] = dashboardTimelineRow(row)
	}
	return result
}

func convertOwnerTimeline(rows []repository.GetUsageDashboardTimelineByOwnerOpenIDRow) []dashboardTimelineRow {
	result := make([]dashboardTimelineRow, len(rows))
	for i, row := range rows {
		result[i] = dashboardTimelineRow(row)
	}
	return result
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
			OwnerOpenid:      l.OwnerOpenid,
			CreatedAt:        timestamppb.New(l.CreatedAt),
		}
	}
	return result
}
