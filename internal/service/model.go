package service

import (
	"context"
	"database/sql"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
	"github.com/HyperToken-dev/fabric/internal/models"
	"github.com/HyperToken-dev/fabric/internal/repository"

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

func (m *ModelService) GetModelInfo(ctx context.Context, req *proto.GetAdminModelInfoRequest) (*proto.GetAdminModelInfoResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	model, err := m.queries.GetModelById(ctx, req.ModelId)
	if err != nil {
		zap.L().Error("get model info failed", zap.Error(err), zap.Int32("model_id", req.ModelId))
		return nil, err
	}
	zap.L().Info("model info retrieved", zap.Int32("model_id", model.ID), zap.String("model_name", model.ModelName), zap.Int32("channel_id", model.ChannelID), zap.Int16("status", model.Status))
	return &proto.GetAdminModelInfoResponse{
		Model: modelToProto(model),
	}, nil
}

func (m *ModelService) CreateModel(ctx context.Context, req *proto.CreateAdminModelRequest) (*proto.CreateAdminModelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
	return &proto.CreateAdminModelResponse{Model: modelToProto(model)}, nil
}

func (m *ModelService) ListModels(ctx context.Context, req *proto.ListAdminModelsRequest) (*proto.ListAdminModelsResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	models, err := m.queries.ListModelsByChannel(ctx, req.ChannelId)
	if err != nil {
		zap.L().Error("list models failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	var modelProtos []*proto.AdminModel = make([]*proto.AdminModel, len(models))
	for idx, model := range models {
		modelProtos[idx] = modelToProto(model)
	}
	zap.L().Info("models listed", zap.Int32("channel_id", req.ChannelId), zap.Int("count", len(modelProtos)))
	return &proto.ListAdminModelsResponse{
		Models: modelProtos,
	}, nil
}

func (m *ModelService) ListCatalogModels(ctx context.Context, req *proto.ListCatalogModelsRequest) (*proto.ListCatalogModelsResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	catalog := models.List(req.ApiFormat)
	modelProtos := make([]*proto.CatalogModel, len(catalog))
	for idx, model := range catalog {
		modelProtos[idx] = &proto.CatalogModel{
			ModelName: model.Name,
			ModelType: model.Type,
		}
	}
	zap.L().Info("catalog models listed", zap.Int32("api_format", req.ApiFormat), zap.Int("count", len(modelProtos)))
	return &proto.ListCatalogModelsResponse{Models: modelProtos}, nil
}

func (m *ModelService) ListClientModels(ctx context.Context, req *proto.ListClientModelsRequest) (*proto.ListClientModelsResponse, error) {
	if _, err := adminauth.RequireUser(ctx); err != nil {
		return nil, err
	}
	rows, err := m.queries.ListClientModels(ctx, req.ChannelName)
	if err != nil {
		zap.L().Error("list client models failed", zap.Error(err), zap.String("channel_name", req.ChannelName))
		return nil, err
	}
	result := make([]*proto.ClientModel, len(rows))
	for i, row := range rows {
		result[i] = &proto.ClientModel{
			ModelName:   row.ModelName,
			ModelType:   row.ModelType,
			ChannelName: row.ChannelName,
		}
	}
	return &proto.ListClientModelsResponse{Models: result}, nil
}

func modelToProto(model repository.Model) *proto.AdminModel {
	return &proto.AdminModel{
		ModelId:   model.ID,
		ModelName: model.ModelName,
		ChannelId: model.ChannelID,
		Status:    int32(model.Status),
		ModelType: model.ModelType,
	}
}
