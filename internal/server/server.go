package server

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	gen "github.com/HyperToken-dev/fabric/gen"
	protoconnect "github.com/HyperToken-dev/fabric/gen/protoconnect"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
	"github.com/HyperToken-dev/fabric/internal/service"
)

// Server implements Connect services used by the management console.
//
// Authorization: Methods that mutate system-wide resources require the admin
// permission either in their service implementation or at this boundary. The
// value is safe for concurrent use after construction because services hold
// sql.DB-backed repositories.
type Server struct {
	protoconnect.UnimplementedApiKeyAdminServiceHandler
	protoconnect.UnimplementedUsageServiceHandler
	protoconnect.UnimplementedChannelAdminServiceHandler
	protoconnect.UnimplementedModelAdminServiceHandler
	protoconnect.UnimplementedIntegralLogServiceHandler
	protoconnect.UnimplementedSensitiveWordServiceHandler

	apiKeySvc      *service.ApiKeyService
	modelSvc       *service.ModelService
	channelSvc     *service.ChannelService
	usageSvc       *service.UsageService
	integralLogSvc *service.IntegralLogService
	sensitiveSvc   *service.SensitiveWordService
}

// ClientServer implements ordinary-user-safe Connect services.
//
// Client responses intentionally expose only user-safe fields, such as channel
// names without provider configuration. Service methods still enforce user
// authentication and ownership scopes on every request.
type ClientServer struct {
	protoconnect.UnimplementedApiKeyClientServiceHandler
	protoconnect.UnimplementedChannelClientServiceHandler
	protoconnect.UnimplementedModelClientServiceHandler

	apiKeySvc  *service.ApiKeyService
	modelSvc   *service.ModelService
	channelSvc *service.ChannelService
}

// NewServer wires administrator service handlers to business services.
func NewServer(apiKeySvc *service.ApiKeyService, channelSvc *service.ChannelService, modelSvc *service.ModelService, usageSvc *service.UsageService, integralLogSvc *service.IntegralLogService, sensitiveSvc *service.SensitiveWordService) *Server {
	return &Server{apiKeySvc: apiKeySvc, channelSvc: channelSvc, modelSvc: modelSvc, usageSvc: usageSvc, integralLogSvc: integralLogSvc, sensitiveSvc: sensitiveSvc}
}

// NewClientServer wires ordinary-user service handlers to business services.
func NewClientServer(apiKeySvc *service.ApiKeyService, channelSvc *service.ChannelService, modelSvc *service.ModelService) *ClientServer {
	return &ClientServer{apiKeySvc: apiKeySvc, channelSvc: channelSvc, modelSvc: modelSvc}
}

// CreateApiKey creates an administrator-owned API key for a channel id.
func (s *Server) CreateApiKey(ctx context.Context, req *connect.Request[gen.CreateAdminApiKeyRequest]) (*connect.Response[gen.CreateAdminApiKeyResponse], error) {
	resp, err := s.apiKeySvc.CreateApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// RevokeApiKey revokes any API key by hash for administrators.
func (s *Server) RevokeApiKey(ctx context.Context, req *connect.Request[gen.RevokeAdminApiKeyRequest]) (*connect.Response[gen.RevokeAdminApiKeyResponse], error) {
	resp, err := s.apiKeySvc.RevokeApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListApiKeysByChannelID lists API keys for an administrator-selected channel.
func (s *Server) ListApiKeysByChannelID(ctx context.Context, req *connect.Request[gen.ListAdminApiKeysByChannelIDRequest]) (*connect.Response[gen.ListAdminApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListApiKeysByChannelID(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListApiKeys lists all administrator-visible API keys with channel names.
func (s *Server) ListApiKeys(ctx context.Context, req *connect.Request[gen.ListAdminApiKeysRequest]) (*connect.Response[gen.ListAdminApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListApiKeys(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListApiKeysByChannelName lists API keys for an administrator-selected channel name.
func (s *Server) ListApiKeysByChannelName(ctx context.Context, req *connect.Request[gen.ListAdminApiKeysByChannelNameRequest]) (*connect.Response[gen.ListAdminApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListApiKeysByChannelName(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListChannels returns administrator channel details.
func (s *Server) ListChannels(ctx context.Context, req *connect.Request[gen.ListAdminChannelsRequest]) (*connect.Response[gen.ListAdminChannelsResponse], error) {
	resp, err := s.channelSvc.ListChannels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListActiveChannels returns administrator channel details for active channels.
func (s *Server) ListActiveChannels(ctx context.Context, req *connect.Request[gen.ListAdminActiveChannelsRequest]) (*connect.Response[gen.ListAdminChannelsResponse], error) {
	resp, err := s.channelSvc.ListActiveChannels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// CreateChannel creates provider-backed channel configuration.
func (s *Server) CreateChannel(ctx context.Context, req *connect.Request[gen.CreateAdminChannelRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.CreateChannel(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateChannelName updates a channel name.
func (s *Server) UpdateChannelName(ctx context.Context, req *connect.Request[gen.UpdateAdminChannelNameRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelName(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateChannelStatus updates a channel lifecycle status.
func (s *Server) UpdateChannelStatus(ctx context.Context, req *connect.Request[gen.UpdateAdminChannelStatusRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelStatus(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateChannelBaseURL updates provider routing configuration.
func (s *Server) UpdateChannelBaseURL(ctx context.Context, req *connect.Request[gen.UpdateAdminChannelBaseURLRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelBaseURL(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateChannelAPIFormat updates provider implementation selection.
func (s *Server) UpdateChannelAPIFormat(ctx context.Context, req *connect.Request[gen.UpdateAdminChannelAPIFormatRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelAPIFormat(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateChannelProviderKey updates write-only provider credentials.
func (s *Server) UpdateChannelProviderKey(ctx context.Context, req *connect.Request[gen.UpdateAdminChannelProviderKeyRequest]) (*connect.Response[gen.AdminChannelResponse], error) {
	resp, err := s.channelSvc.UpdateChannelProviderKey(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetModelInfo returns administrator model details.
func (s *Server) GetModelInfo(ctx context.Context, req *connect.Request[gen.GetAdminModelInfoRequest]) (*connect.Response[gen.GetAdminModelInfoResponse], error) {
	resp, err := s.modelSvc.GetModelInfo(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// CreateModel creates administrator-managed model configuration.
func (s *Server) CreateModel(ctx context.Context, req *connect.Request[gen.CreateAdminModelRequest]) (*connect.Response[gen.CreateAdminModelResponse], error) {
	resp, err := s.modelSvc.CreateModel(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListModels lists administrator model details by channel id.
func (s *Server) ListModels(ctx context.Context, req *connect.Request[gen.ListAdminModelsRequest]) (*connect.Response[gen.ListAdminModelsResponse], error) {
	resp, err := s.modelSvc.ListModels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListCatalogModels lists built-in model catalogs for administrators.
func (s *Server) ListCatalogModels(ctx context.Context, req *connect.Request[gen.ListCatalogModelsRequest]) (*connect.Response[gen.ListCatalogModelsResponse], error) {
	resp, err := s.modelSvc.ListCatalogModels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageByKeyHash returns administrator usage rows by key hash.
func (s *Server) GetUsageByKeyHash(ctx context.Context, req *connect.Request[gen.GetUsageByKeyHashRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByKeyHash(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageByKeyID returns administrator usage rows by key id.
func (s *Server) GetUsageByKeyID(ctx context.Context, req *connect.Request[gen.GetUsageByKeyIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByKeyID(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageByChannelID returns administrator usage rows by channel id.
func (s *Server) GetUsageByChannelID(ctx context.Context, req *connect.Request[gen.GetUsageByChannelIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByChannelID(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageByModelID returns administrator usage rows by model id.
func (s *Server) GetUsageByModelID(ctx context.Context, req *connect.Request[gen.GetUsageByModelIDRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByModelID(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageByDeadlineAndKeyHash returns administrator usage stats by key hash.
func (s *Server) GetUsageByDeadlineAndKeyHash(ctx context.Context, req *connect.Request[gen.GetUsageByDeadlineAndKeyHashRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageByDeadlineAndKeyHash(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageSummary returns administrator global usage summary.
func (s *Server) GetUsageSummary(ctx context.Context, req *connect.Request[gen.GetUsageSummaryRequest]) (*connect.Response[gen.GetUsageResponse], error) {
	resp, err := s.usageSvc.GetUsageSummary(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetUsageDashboard returns role-scoped dashboard metrics.
func (s *Server) GetUsageDashboard(ctx context.Context, req *connect.Request[gen.GetUsageDashboardRequest]) (*connect.Response[gen.GetUsageDashboardResponse], error) {
	resp, err := s.usageSvc.GetUsageDashboard(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// CreateIntegralLog is admin-only because integral logs include audit data.
func (s *Server) CreateIntegralLog(ctx context.Context, req *connect.Request[gen.CreateIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.integralLogSvc.CreateIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetIntegralLog returns one owner-scoped integral log, or any log for admins.
func (s *Server) GetIntegralLog(ctx context.Context, req *connect.Request[gen.GetIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	resp, err := s.integralLogSvc.GetIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// ListIntegralLogs returns owner-scoped logs, or global/filterable logs for admins.
func (s *Server) ListIntegralLogs(ctx context.Context, req *connect.Request[gen.ListIntegralLogsRequest]) (*connect.Response[gen.ListIntegralLogsResponse], error) {
	resp, err := s.integralLogSvc.ListIntegralLogs(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateIntegralLog is admin-only because integral logs include audit data.
func (s *Server) UpdateIntegralLog(ctx context.Context, req *connect.Request[gen.UpdateIntegralLogRequest]) (*connect.Response[gen.IntegralLogResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.integralLogSvc.UpdateIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// DeleteIntegralLog is admin-only because integral logs include audit data.
func (s *Server) DeleteIntegralLog(ctx context.Context, req *connect.Request[gen.DeleteIntegralLogRequest]) (*connect.Response[gen.DeleteIntegralLogResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.integralLogSvc.DeleteIntegralLog(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// GetSensitiveWordStatus is admin-only because sensitive policy is global.
func (s *Server) GetSensitiveWordStatus(ctx context.Context, req *connect.Request[gen.GetSensitiveWordStatusRequest]) (*connect.Response[gen.GetSensitiveWordStatusResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.GetSensitiveWordStatus(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateSensitiveWordEnabled is admin-only because sensitive policy is global.
func (s *Server) UpdateSensitiveWordEnabled(ctx context.Context, req *connect.Request[gen.UpdateSensitiveWordEnabledRequest]) (*connect.Response[gen.GetSensitiveWordStatusResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.UpdateSensitiveWordEnabled(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

func connectError(err error) error {
	message := err.Error()
	if strings.Contains(message, "authenticated user is required") {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	if strings.Contains(message, "admin role is required") || strings.Contains(message, "admin permission is required") {
		return connect.NewError(connect.CodePermissionDenied, err)
	}
	if strings.Contains(message, "active channel is required") {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// ListSensitiveDictionaries is admin-only because dictionaries are global policy.
func (s *Server) ListSensitiveDictionaries(ctx context.Context, req *connect.Request[gen.ListSensitiveDictionariesRequest]) (*connect.Response[gen.ListSensitiveDictionariesResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.ListSensitiveDictionaries(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// GetSensitiveDictionary is admin-only because dictionaries are global policy.
func (s *Server) GetSensitiveDictionary(ctx context.Context, req *connect.Request[gen.GetSensitiveDictionaryRequest]) (*connect.Response[gen.GetSensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.GetSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// CreateSensitiveDictionary is admin-only because dictionaries are global policy.
func (s *Server) CreateSensitiveDictionary(ctx context.Context, req *connect.Request[gen.CreateSensitiveDictionaryRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.CreateSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateSensitiveDictionaryEffectModels is admin-only because dictionaries are global policy.
func (s *Server) UpdateSensitiveDictionaryEffectModels(ctx context.Context, req *connect.Request[gen.UpdateSensitiveDictionaryEffectModelsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.UpdateSensitiveDictionaryEffectModels(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// UpdateSensitiveDictionaryEnabled is admin-only because dictionaries are global policy.
func (s *Server) UpdateSensitiveDictionaryEnabled(ctx context.Context, req *connect.Request[gen.UpdateSensitiveDictionaryEnabledRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.UpdateSensitiveDictionaryEnabled(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// AddSensitiveWords is admin-only because dictionaries are global policy.
func (s *Server) AddSensitiveWords(ctx context.Context, req *connect.Request[gen.AddSensitiveWordsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.AddSensitiveWords(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// RemoveSensitiveWords is admin-only because dictionaries are global policy.
func (s *Server) RemoveSensitiveWords(ctx context.Context, req *connect.Request[gen.RemoveSensitiveWordsRequest]) (*connect.Response[gen.SensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.RemoveSensitiveWords(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// DeleteSensitiveDictionary is admin-only because dictionaries are global policy.
func (s *Server) DeleteSensitiveDictionary(ctx context.Context, req *connect.Request[gen.DeleteSensitiveDictionaryRequest]) (*connect.Response[gen.DeleteSensitiveDictionaryResponse], error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	resp, err := s.sensitiveSvc.DeleteSensitiveDictionary(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// Client API methods.
func (s *ClientServer) CreateApiKey(ctx context.Context, req *connect.Request[gen.CreateClientApiKeyRequest]) (*connect.Response[gen.CreateClientApiKeyResponse], error) {
	resp, err := s.apiKeySvc.CreateClientApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *ClientServer) RevokeApiKey(ctx context.Context, req *connect.Request[gen.RevokeClientApiKeyRequest]) (*connect.Response[gen.RevokeClientApiKeyResponse], error) {
	resp, err := s.apiKeySvc.RevokeClientApiKey(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *ClientServer) ListApiKeys(ctx context.Context, req *connect.Request[gen.ListClientApiKeysRequest]) (*connect.Response[gen.ListClientApiKeysResponse], error) {
	resp, err := s.apiKeySvc.ListClientApiKeys(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *ClientServer) ListChannels(ctx context.Context, req *connect.Request[gen.ListClientChannelsRequest]) (*connect.Response[gen.ListClientChannelsResponse], error) {
	resp, err := s.channelSvc.ListClientChannels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *ClientServer) ListModels(ctx context.Context, req *connect.Request[gen.ListClientModelsRequest]) (*connect.Response[gen.ListClientModelsResponse], error) {
	resp, err := s.modelSvc.ListClientModels(ctx, req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(resp), nil
}
