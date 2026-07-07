package service

import (
	"context"
	"database/sql"
	proto "hyper-token/gen"
	"hyper-token/internal/repository"
)

type ModelService struct {
	queries *repository.Queries
}

func NewModelService(db *sql.DB) *ModelService {
	return &ModelService{
		queries: repository.New(db),
	}
}

func (m *ModelService) GetModelInfo(ctx context.Context, req *proto.GetModelInfoRequest) (*proto.GetModelInfoResponse, error) {
	model, err := m.queries.GetModelById(ctx, req.ModelId)
	if err != nil {
		return nil, err
	}
	return &proto.GetModelInfoResponse{
		Model: &proto.Model{
			ModelId:   model.ID,
			ModelName: model.ModelName,
			ChannelId: model.ChannelID,
		},
	}, nil
}

func (m *ModelService) ListModels(ctx context.Context, req *proto.ListModelsRequest) (*proto.ListModelsResponse, error) {
	models, err := m.queries.ListModelsByChannel(ctx, req.ChannelId)
	if err != nil {
		return nil, err
	}
	var modelProtos []*proto.Model = make([]*proto.Model, len(models))
	for idx, model := range models {
		modelProtos[idx] = &proto.Model{
			ModelId:   model.ID,
			ModelName: model.ModelName,
			ChannelId: model.ChannelID,
		}
	}
	return &proto.ListModelsResponse{
		Models: modelProtos,
	}, nil
}
