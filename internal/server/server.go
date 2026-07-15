package server

import (
	"context"

	gen "fabric/gen"
	protoconnect "fabric/gen/protoconnect"
	"fabric/internal/service"

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

func NewServer(apiKeySvc *service.ApiKeyService, channelSvc *service.ChannelService, modelSvc *service.ModelService, usageSvc *service.UsageService) *Server {
	return &Server{
		apiKeySvc:  apiKeySvc,
		channelSvc: channelSvc,
		modelSvc:   modelSvc,
		usageSvc:   usageSvc,
	}
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

func (s *Server) ListActiveChannels(ctx context.Context, req *connect.Request[gen.ListActiveChannelsRequest]) (*connect.Response[gen.ListChannelsResponse], error) {
	resp, err := s.channelSvc.ListActiveChannels(ctx, req.Msg)
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

func (s *Server) UpdateChannelName(ctx context.Context, req *connect.Request[gen.UpdateChannelNameRequest]) (*connect.Response[gen.UpdateChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelName(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateChannelStatus(ctx context.Context, req *connect.Request[gen.UpdateChannelStatusRequest]) (*connect.Response[gen.UpdateChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelStatus(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateChannelBaseURL(ctx context.Context, req *connect.Request[gen.UpdateChannelBaseURLRequest]) (*connect.Response[gen.UpdateChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelBaseURL(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateChannelAPIFormat(ctx context.Context, req *connect.Request[gen.UpdateChannelAPIFormatRequest]) (*connect.Response[gen.UpdateChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelAPIFormat(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateChannelProviderKey(ctx context.Context, req *connect.Request[gen.UpdateChannelProviderKeyRequest]) (*connect.Response[gen.UpdateChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelProviderKey(ctx, req.Msg)
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

func (s *Server) CreateModel(ctx context.Context, req *connect.Request[gen.CreateModelRequest]) (*connect.Response[gen.CreateModelResponse], error) {
	resp, err := s.modelSvc.CreateModel(ctx, req.Msg)
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
