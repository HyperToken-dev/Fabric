package server

import (
	"context"

	gen "github.com/HyperToken-dev/fabric/gen"
	protoconnect "github.com/HyperToken-dev/fabric/gen/protoconnect"
	"github.com/HyperToken-dev/fabric/internal/service"

	"connectrpc.com/connect"
)

type Server struct {
	protoconnect.UnimplementedManageApiKeyServiceHandler
	protoconnect.UnimplementedUsageServiceHandler
	protoconnect.UnimplementedChannelServiceHandler
	protoconnect.UnimplementedModelServiceHandler
	protoconnect.UnimplementedIntegralLogServiceHandler
	protoconnect.UnimplementedSensitiveWordServiceHandler

	apiKeySvc      *service.ApiKeyService
	modelSvc       *service.ModelService
	channelSvc     *service.ChannelService
	usageSvc       *service.UsageService
	integralLogSvc *service.IntegralLogService
	sensitiveSvc   *service.SensitiveWordService
}

func NewServer(apiKeySvc *service.ApiKeyService, channelSvc *service.ChannelService, modelSvc *service.ModelService, usageSvc *service.UsageService, integralLogSvc *service.IntegralLogService, sensitiveSvc *service.SensitiveWordService) *Server {
	return &Server{
		apiKeySvc:      apiKeySvc,
		channelSvc:     channelSvc,
		modelSvc:       modelSvc,
		usageSvc:       usageSvc,
		integralLogSvc: integralLogSvc,
		sensitiveSvc:   sensitiveSvc,
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

func (s *Server) ListCatalogModels(ctx context.Context, req *connect.Request[gen.ListCatalogModelsRequest]) (*connect.Response[gen.ListCatalogModelsResponse], error) {
	resp, err := s.modelSvc.ListCatalogModels(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageByKeyHash(ctx context.Context, req *connect.Request[gen.GetUsageByKeyHashRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByKeyHash(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageByKeyID(ctx context.Context, req *connect.Request[gen.GetUsageByKeyIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByKeyID(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageByChannelID(ctx context.Context, req *connect.Request[gen.GetUsageByChannelIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByChannelID(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageByModelID(ctx context.Context, req *connect.Request[gen.GetUsageByModelIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByModelID(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageByDeadlineAndKeyHash(ctx context.Context, req *connect.Request[gen.GetUsageByDeadlineAndKeyHashRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByDeadlineAndKeyHash(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageSummary(ctx context.Context, req *connect.Request[gen.GetUsageSummaryRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageSummary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetUsageDashboard(ctx context.Context, req *connect.Request[gen.GetUsageDashboardRequest]) (*connect.Response[gen.GetUsageDashboardResponse], error) {
	resp, err := s.usageSvc.GetUsageDashboard(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) CreateIntegralLog(ctx context.Context, req *connect.Request[gen.CreateIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	resp, err := s.integralLogSvc.CreateIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetIntegralLog(ctx context.Context, req *connect.Request[gen.GetIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	resp, err := s.integralLogSvc.GetIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListIntegralLogs(ctx context.Context, req *connect.Request[gen.ListIntegralLogsRequest]) (*connect.Response[gen.ListIntegralLogsResponse], error) {
	resp, err := s.integralLogSvc.ListIntegralLogs(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateIntegralLog(ctx context.Context, req *connect.Request[gen.UpdateIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	resp, err := s.integralLogSvc.UpdateIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) DeleteIntegralLog(ctx context.Context, req *connect.Request[gen.DeleteIntegralLogRequest]) (*connect.Response[gen.DeleteIntegralLogResponse], error) {
	resp, err := s.integralLogSvc.DeleteIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetSensitiveWordStatus(ctx context.Context, req *connect.Request[gen.GetSensitiveWordStatusRequest]) (*connect.Response[gen.GetSensitiveWordStatusResponse], error) {
	resp, err := s.sensitiveSvc.GetSensitiveWordStatus(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateSensitiveWordEnabled(ctx context.Context, req *connect.Request[gen.UpdateSensitiveWordEnabledRequest]) (*connect.Response[gen.GetSensitiveWordStatusResponse], error) {
	resp, err := s.sensitiveSvc.UpdateSensitiveWordEnabled(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ListSensitiveDictionaries(ctx context.Context, req *connect.Request[gen.ListSensitiveDictionariesRequest]) (*connect.Response[gen.ListSensitiveDictionariesResponse], error) {
	resp, err := s.sensitiveSvc.ListSensitiveDictionaries(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetSensitiveDictionary(ctx context.Context, req *connect.Request[gen.GetSensitiveDictionaryRequest]) (*connect.Response[gen.GetSensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.GetSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) CreateSensitiveDictionary(ctx context.Context, req *connect.Request[gen.CreateSensitiveDictionaryRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.CreateSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateSensitiveDictionaryEffectModels(ctx context.Context, req *connect.Request[gen.UpdateSensitiveDictionaryEffectModelsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.UpdateSensitiveDictionaryEffectModels(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) UpdateSensitiveDictionaryEnabled(ctx context.Context, req *connect.Request[gen.UpdateSensitiveDictionaryEnabledRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.UpdateSensitiveDictionaryEnabled(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) AddSensitiveWords(ctx context.Context, req *connect.Request[gen.AddSensitiveWordsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.AddSensitiveWords(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) RemoveSensitiveWords(ctx context.Context, req *connect.Request[gen.RemoveSensitiveWordsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.RemoveSensitiveWords(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) DeleteSensitiveDictionary(ctx context.Context, req *connect.Request[gen.DeleteSensitiveDictionaryRequest]) (*connect.Response[gen.DeleteSensitiveDictionaryResponse], error) {
	resp, err := s.sensitiveSvc.DeleteSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}
