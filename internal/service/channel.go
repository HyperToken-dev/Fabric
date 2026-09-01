package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/adminauth"
	"github.com/HyperToken-dev/fabric/internal/repository"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChannelService struct {
	queries *repository.Queries
}

const (
	channelNameMaxLength    = 20
	channelBaseURLMaxLength = 100

	channelStatusActive  int32 = 1
	channelStatusBanned  int32 = 2
	channelStatusPending int32 = 3
)

func NewChannelService(db *sql.DB) *ChannelService {
	return &ChannelService{queries: repository.New(db)}
}

func (s *ChannelService) ListChannels(ctx context.Context, req *proto.ListAdminChannelsRequest) (*proto.ListAdminChannelsResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListChannels(ctx)
	if err != nil {
		zap.L().Error("list channels failed", zap.Error(err))
		return nil, err
	}

	channels := make([]*proto.AdminChannel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
	zap.L().Info("channels listed", zap.Int("count", len(channels)))
	return &proto.ListAdminChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) ListActiveChannels(ctx context.Context, req *proto.ListAdminActiveChannelsRequest) (*proto.ListAdminChannelsResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListActiveChannels(ctx)
	if err != nil {
		zap.L().Error("list active channels failed", zap.Error(err))
		return nil, err
	}

	channels := make([]*proto.AdminChannel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
	zap.L().Info("active channels listed", zap.Int("count", len(channels)))
	return &proto.ListAdminChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) CreateChannel(ctx context.Context, req *proto.CreateAdminChannelRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := validateChannelName(req.ChannelName); err != nil {
		return nil, err
	}
	if err := validateChannelBaseURL(req.BaseUrl); err != nil {
		return nil, err
	}

	repoChannel, err := s.queries.CreateChannel(ctx, repository.CreateChannelParams{
		ChannelName: req.ChannelName,
		BaseUrl:     req.BaseUrl,
		ProviderKey: req.ProviderKey,
		ApiFormat:   req.ApiFormat,
	})
	if err != nil {
		zap.L().Error("create channel failed", zap.Error(err), zap.String("channel_name", req.ChannelName), zap.String("base_url", req.BaseUrl), zap.Int32("api_format", req.ApiFormat))
		return nil, err
	}
	zap.L().Info("channel created", zap.Int32("channel_id", repoChannel.ID), zap.String("channel_name", repoChannel.ChannelName), zap.String("base_url", repoChannel.BaseUrl), zap.Int32("api_format", repoChannel.ApiFormat))
	return &proto.AdminChannelResponse{
		Channel: channelToProto(repoChannel),
	}, nil
}

func (s *ChannelService) UpdateChannelName(ctx context.Context, req *proto.UpdateAdminChannelNameRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := validateChannelName(req.ChannelName); err != nil {
		return nil, err
	}

	repoChannel, err := s.queries.UpdateChannelName(ctx, repository.UpdateChannelNameParams{
		ID:          req.ChannelId,
		ChannelName: req.ChannelName,
	})
	if err != nil {
		zap.L().Error("update channel name failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.String("channel_name", req.ChannelName))
		return nil, err
	}
	zap.L().Info("channel name updated", zap.Int32("channel_id", repoChannel.ID), zap.String("channel_name", repoChannel.ChannelName))
	return &proto.AdminChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelStatus(ctx context.Context, req *proto.UpdateAdminChannelStatusRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := validateChannelStatus(req.Status); err != nil {
		return nil, err
	}

	repoChannel, err := s.queries.UpdateChannelStatus(ctx, repository.UpdateChannelStatusParams{
		ID:     req.ChannelId,
		Status: int16(req.Status),
	})
	if err != nil {
		zap.L().Error("update channel status failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.Int32("status", req.Status))
		return nil, err
	}
	zap.L().Info("channel status updated", zap.Int32("channel_id", repoChannel.ID), zap.Int16("status", repoChannel.Status))
	return &proto.AdminChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelBaseURL(ctx context.Context, req *proto.UpdateAdminChannelBaseURLRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := validateChannelBaseURL(req.BaseUrl); err != nil {
		return nil, err
	}

	repoChannel, err := s.queries.UpdateChannelBaseURL(ctx, repository.UpdateChannelBaseURLParams{
		ID:      req.ChannelId,
		BaseUrl: req.BaseUrl,
	})
	if err != nil {
		zap.L().Error("update channel base url failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.String("base_url", req.BaseUrl))
		return nil, err
	}
	zap.L().Info("channel base url updated", zap.Int32("channel_id", repoChannel.ID), zap.String("base_url", repoChannel.BaseUrl))
	return &proto.AdminChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelAPIFormat(ctx context.Context, req *proto.UpdateAdminChannelAPIFormatRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	repoChannel, err := s.queries.UpdateChannelAPIFormat(ctx, repository.UpdateChannelAPIFormatParams{
		ID:        req.ChannelId,
		ApiFormat: req.ApiFormat,
	})
	if err != nil {
		zap.L().Error("update channel api format failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.Int32("api_format", req.ApiFormat))
		return nil, err
	}
	zap.L().Info("channel api format updated", zap.Int32("channel_id", repoChannel.ID), zap.Int32("api_format", repoChannel.ApiFormat))
	return &proto.AdminChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelProviderKey(ctx context.Context, req *proto.UpdateAdminChannelProviderKeyRequest) (*proto.AdminChannelResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	repoChannel, err := s.queries.UpdateChannelProviderKey(ctx, repository.UpdateChannelProviderKeyParams{
		ID:          req.ChannelId,
		ProviderKey: req.ProviderKey,
	})
	if err != nil {
		zap.L().Error("update channel provider key failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	zap.L().Info("channel provider key updated", zap.Int32("channel_id", repoChannel.ID))
	return &proto.AdminChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) ListClientChannels(ctx context.Context, req *proto.ListClientChannelsRequest) (*proto.ListClientChannelsResponse, error) {
	if _, err := adminauth.RequireUser(ctx); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListActiveChannelNames(ctx)
	if err != nil {
		zap.L().Error("list client channels failed", zap.Error(err))
		return nil, err
	}
	channels := make([]*proto.ClientChannel, len(rows))
	for i, name := range rows {
		channels[i] = &proto.ClientChannel{ChannelName: name}
	}
	return &proto.ListClientChannelsResponse{Channels: channels}, nil
}

func validateChannelName(channelName string) error {
	if strings.TrimSpace(channelName) == "" {
		return ValidationError{Message: "channel name is required"}
	}
	if len(channelName) > channelNameMaxLength {
		return ValidationError{Message: fmt.Sprintf("channel name must be at most %d characters", channelNameMaxLength)}
	}
	return nil
}

func validateChannelBaseURL(baseURL string) error {
	if len(baseURL) > channelBaseURLMaxLength {
		return ValidationError{Message: fmt.Sprintf("base_url must be at most %d characters", channelBaseURLMaxLength)}
	}
	if _, err := coreproxy.ParseBaseURL(baseURL); err != nil {
		return ValidationError{Message: fmt.Sprintf("invalid base_url: %v", err)}
	}
	return nil
}

func validateChannelStatus(status int32) error {
	switch status {
	case channelStatusActive, channelStatusBanned, channelStatusPending:
		return nil
	default:
		return ValidationError{Message: fmt.Sprintf("invalid channel status: %d", status)}
	}
}

func channelToProto(channel repository.Channel) *proto.AdminChannel {
	return &proto.AdminChannel{
		ChannelId:   channel.ID,
		ChannelName: channel.ChannelName,
		CreatedAt:   timestamppb.New(channel.CreatedAt),
		Status:      int32(channel.Status),
		BaseUrl:     channel.BaseUrl,
		ApiFormat:   channel.ApiFormat,
	}
}
