package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IntegralLogService struct {
	queries *repository.Queries
}

const (
	integralLogDefaultLimit = 100
	integralLogMaxLimit     = 500
)

func NewIntegralLogService(db *sql.DB) *IntegralLogService {
	return &IntegralLogService{queries: repository.New(db)}
}

func (s *IntegralLogService) CreateIntegralLog(ctx context.Context, req *proto.CreateIntegralLogRequest) (*proto.IntegralLogResponse, error) {
	if req.KeyId <= 0 {
		return nil, fmt.Errorf("key_id is required")
	}
	contextJSON, err := validateIntegralLogContext(req.Context)
	if err != nil {
		return nil, err
	}

	log, err := s.queries.CreateIntegralLog(ctx, repository.CreateIntegralLogParams{
		Context:  contextJSON,
		Response: integralLogResponse(req.Response),
		KeyID:    req.KeyId,
	})
	if err != nil {
		zap.L().Error("create integral log failed", zap.Error(err), zap.Int32("key_id", req.KeyId))
		return nil, err
	}
	zap.L().Info("integral log created", zap.Int32("integral_log_id", log.ID), zap.Int32("key_id", log.KeyID))
	return &proto.IntegralLogResponse{Log: integralLogToProto(log)}, nil
}

func (s *IntegralLogService) GetIntegralLog(ctx context.Context, req *proto.GetIntegralLogRequest) (*proto.IntegralLogResponse, error) {
	if req.Id <= 0 {
		return nil, fmt.Errorf("id is required")
	}

	log, err := s.queries.GetIntegralLogByID(ctx, req.Id)
	if err != nil {
		zap.L().Error("get integral log failed", zap.Error(err), zap.Int32("integral_log_id", req.Id))
		return nil, err
	}
	zap.L().Info("integral log retrieved", zap.Int32("integral_log_id", log.ID), zap.Int32("key_id", log.KeyID))
	return &proto.IntegralLogResponse{Log: integralLogToProto(log)}, nil
}

func (s *IntegralLogService) ListIntegralLogs(ctx context.Context, req *proto.ListIntegralLogsRequest) (*proto.ListIntegralLogsResponse, error) {
	limit, offset := normalizeIntegralLogPagination(req.Limit, req.Offset)

	var (
		logs  []repository.IntegralLog
		total int64
		err   error
	)
	if req.KeyId > 0 {
		logs, err = s.queries.ListIntegralLogsByKeyID(ctx, repository.ListIntegralLogsByKeyIDParams{
			KeyID:  req.KeyId,
			Limit:  limit,
			Offset: offset,
		})
		if err == nil {
			total, err = s.queries.CountIntegralLogsByKeyID(ctx, req.KeyId)
		}
	} else {
		logs, err = s.queries.ListIntegralLogs(ctx, repository.ListIntegralLogsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err == nil {
			total, err = s.queries.CountIntegralLogs(ctx)
		}
	}
	if err != nil {
		zap.L().Error("list integral logs failed", zap.Error(err), zap.Int32("key_id", req.KeyId), zap.Int32("limit", limit), zap.Int32("offset", offset))
		return nil, err
	}

	zap.L().Info("integral logs listed", zap.Int32("key_id", req.KeyId), zap.Int("count", len(logs)), zap.Int64("total", total))
	return &proto.ListIntegralLogsResponse{Logs: integralLogsToProto(logs), Total: total}, nil
}

func (s *IntegralLogService) UpdateIntegralLog(ctx context.Context, req *proto.UpdateIntegralLogRequest) (*proto.IntegralLogResponse, error) {
	if req.Id <= 0 {
		return nil, fmt.Errorf("id is required")
	}
	contextJSON, err := validateIntegralLogContext(req.Context)
	if err != nil {
		return nil, err
	}

	log, err := s.queries.UpdateIntegralLog(ctx, repository.UpdateIntegralLogParams{
		ID:       req.Id,
		Context:  contextJSON,
		Response: integralLogResponse(req.Response),
	})
	if err != nil {
		zap.L().Error("update integral log failed", zap.Error(err), zap.Int32("integral_log_id", req.Id))
		return nil, err
	}
	zap.L().Info("integral log updated", zap.Int32("integral_log_id", log.ID), zap.Int32("key_id", log.KeyID))
	return &proto.IntegralLogResponse{Log: integralLogToProto(log)}, nil
}

func (s *IntegralLogService) DeleteIntegralLog(ctx context.Context, req *proto.DeleteIntegralLogRequest) (*proto.DeleteIntegralLogResponse, error) {
	if req.Id <= 0 {
		return nil, fmt.Errorf("id is required")
	}

	if err := s.queries.DeleteIntegralLog(ctx, req.Id); err != nil {
		zap.L().Error("delete integral log failed", zap.Error(err), zap.Int32("integral_log_id", req.Id))
		return nil, err
	}
	zap.L().Info("integral log deleted", zap.Int32("integral_log_id", req.Id))
	return &proto.DeleteIntegralLogResponse{}, nil
}

func validateIntegralLogContext(contextText string) (json.RawMessage, error) {
	contextText = strings.TrimSpace(contextText)
	if contextText == "" {
		return nil, fmt.Errorf("context is required")
	}
	contextJSON := json.RawMessage(contextText)
	if !json.Valid(contextJSON) {
		return nil, fmt.Errorf("context must be valid JSON")
	}
	return contextJSON, nil
}

func integralLogResponse(response string) sql.NullString {
	if response == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: response, Valid: true}
}

func normalizeIntegralLogPagination(limit, offset int32) (int32, int32) {
	if limit <= 0 {
		limit = integralLogDefaultLimit
	}
	if limit > integralLogMaxLimit {
		limit = integralLogMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func integralLogsToProto(logs []repository.IntegralLog) []*proto.IntegralLog {
	result := make([]*proto.IntegralLog, len(logs))
	for i, log := range logs {
		result[i] = integralLogToProto(log)
	}
	return result
}

func integralLogToProto(log repository.IntegralLog) *proto.IntegralLog {
	response := ""
	if log.Response.Valid {
		response = log.Response.String
	}
	return &proto.IntegralLog{
		Id:        log.ID,
		Context:   string(log.Context),
		Response:  response,
		KeyId:     log.KeyID,
		CreatedAt: timestamppb.New(log.CreatedAt),
	}
}
