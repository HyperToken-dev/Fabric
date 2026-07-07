package server

import (
	"context"

	gen "hyper-token/gen"
	protoconnect "hyper-token/gen/protoconnect"
	"hyper-token/internal/service"

	"connectrpc.com/connect"
)

type Server struct {
	protoconnect.UnimplementedManageApiKeyServiceHandler
	protoconnect.UnimplementedUsageServiceHandler
	protoconnect.UnimplementedChannelServiceHandler
	protoconnect.UnimplementedModelServiceHandler

	apiKeySvc  *service.ApiKeyService
	modelSvc   *service.ModelService
	channelSvc *service.ChannelService
	usageSvc   *service.UsageService
}

func NewServer(apiKeySvc *service.ApiKeyService) *Server {
	return &Server{apiKeySvc: apiKeySvc}
}

func (s *Server) CreateApiKey(ctx context.Context, req *connect.Request[gen.CreateApiKeyRequest]) (*connect.Response[gen.CreateApiKeyResponse], error) {
	resp, err := s.apiKeySvc.CreateApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) RevokeApiKey(ctx context.Context, req *connect.Request[gen.RevokeApiKeyRequest]) (*connect.Response[gen.RevokeApiKeyResponse], error) {
	resp, err := s.apiKeySvc.RevokeApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListApiKeysByChannelID(ctx context.Context, req *connect.Request[gen.ListApiKeysByChannelIDRequest]) (*connect.Response[gen.ListApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListApiKeysByChannelID(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListApiKeysByChannelName(ctx context.Context, req *connect.Request[gen.ListApiKeysByChannelNameRequest]) (*connect.Response[gen.ListApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListApiKeysByChannelName(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListChannels(ctx context.Context, req *connect.Request[gen.ListChannelsRequest]) (*connect.Response[gen.ListChannelsResponse], error) {
	resp, err := s.channelSvc.ListChannels(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) CreateChannel(ctx context.Context, req *connect.Request[gen.CreateChannelRequest]) (*connect.Response[gen.CreateChannelResponse], error) {
	resp, err := s.channelSvc.CreateChannel(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetModelInfo(ctx context.Context, req *connect.Request[gen.GetModelInfoRequest]) (*connect.Response[gen.GetModelInfoResponse], error) {
	resp, err := s.modelSvc.GetModelInfo(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListModels(ctx context.Context, req *connect.Request[gen.ListModelsRequest]) (*connect.Response[gen.ListModelsResponse], error) {
	resp, err := s.modelSvc.ListModels(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
