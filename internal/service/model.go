package service

import (
	"context"
	"database/sql"

	proto "fabric/gen"
	"fabric/internal/repository"

	"go.uber.org/zap"
)

type ModelService struct {
	queries *repository.Queries
}

const (
	modelStatusActive int16 = 1
	modelTypeText     int32 = 1
)

func NewModelService(db *sql.DB) *ModelService {
	return &ModelService{
		queries: repository.New(db),
	}
}

func (m *ModelService) GetModelInfo(ctx context.Context, req *proto.GetModelInfoRequest) (*proto.GetModelInfoResponse, error) {
	model, err := m.queries.GetModelById(ctx, req.ModelId)
	if err != nil {
		zap.L().Error("get model info failed", zap.Error(err), zap.Int32("model_id", req.ModelId))
		return nil, err
	}
	zap.L().Info("model info retrieved", zap.Int32("model_id", model.ID), zap.String("model_name", model.ModelName), zap.Int32("channel_id", model.ChannelID), zap.Int16("status", model.Status))
	return &proto.GetModelInfoResponse{
		Model: modelToProto(model),
	}, nil
}

func (m *ModelService) CreateModel(ctx context.Context, req *proto.CreateModelRequest) (*proto.CreateModelResponse, error) {
	status := int16(req.Status)
	if status == 0 {
		status = modelStatusActive
	}
	modelType := req.ModelType
	if modelType == 0 {
		modelType = modelTypeText
	}

	model, err := m.queries.CreateModel(ctx, repository.CreateModelParams{
		ChannelID: req.ChannelId,
		ModelName: req.ModelName,
		Status:    status,
		ModelType: modelType,
	})
	if err != nil {
		zap.L().Error("create model failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.String("model_name", req.ModelName), zap.Int16("status", status), zap.Int32("model_type", modelType))
		return nil, err
	}
	zap.L().Info("model created", zap.Int32("model_id", model.ID), zap.String("model_name", model.ModelName), zap.Int32("channel_id", model.ChannelID), zap.Int16("status", model.Status), zap.Int32("model_type", model.ModelType))
	return &proto.CreateModelResponse{Model: modelToProto(model)}, nil
}

func (m *ModelService) ListModels(ctx context.Context, req *proto.ListModelsRequest) (*proto.ListModelsResponse, error) {
	models, err := m.queries.ListModelsByChannel(ctx, req.ChannelId)
	if err != nil {
		zap.L().Error("list models failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	var modelProtos []*proto.Model = make([]*proto.Model, len(models))
	for idx, model := range models {
		modelProtos[idx] = modelToProto(model)
	}
	zap.L().Info("models listed", zap.Int32("channel_id", req.ChannelId), zap.Int("count", len(modelProtos)))
	return &proto.ListModelsResponse{
		Models: modelProtos,
	}, nil
}

func modelToProto(model repository.Model) *proto.Model {
	return &proto.Model{
		ModelId:   model.ID,
		ModelName: model.ModelName,
		ChannelId: model.ChannelID,
		Status:    int32(model.Status),
		ModelType: model.ModelType,
	}
}
