package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	coreopenai "github.com/HyperToken-dev/fabric/core/providers/openai"
	proto "github.com/HyperToken-dev/fabric/gen"
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

func (s *ChannelService) ListChannels(ctx context.Context, req *proto.ListChannelsRequest) (*proto.ListChannelsResponse, error) {
	rows, err := s.queries.ListChannels(ctx)
	if err != nil {
		zap.L().Error("list channels failed", zap.Error(err))
		return nil, err
	}

	channels := make([]*proto.Channel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
	zap.L().Info("channels listed", zap.Int("count", len(channels)))
	return &proto.ListChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) ListActiveChannels(ctx context.Context, req *proto.ListActiveChannelsRequest) (*proto.ListChannelsResponse, error) {
	rows, err := s.queries.ListActiveChannels(ctx)
	if err != nil {
		zap.L().Error("list active channels failed", zap.Error(err))
		return nil, err
	}

	channels := make([]*proto.Channel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
	zap.L().Info("active channels listed", zap.Int("count", len(channels)))
	return &proto.ListChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) CreateChannel(ctx context.Context, req *proto.CreateChannelRequest) (*proto.CreateChannelResponse, error) {
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
	return &proto.CreateChannelResponse{
		Channel: channelToProto(repoChannel),
	}, nil
}

func (s *ChannelService) UpdateChannelName(ctx context.Context, req *proto.UpdateChannelNameRequest) (*proto.UpdateChannelResponse, error) {
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
	return &proto.UpdateChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelStatus(ctx context.Context, req *proto.UpdateChannelStatusRequest) (*proto.UpdateChannelResponse, error) {
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
	return &proto.UpdateChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelBaseURL(ctx context.Context, req *proto.UpdateChannelBaseURLRequest) (*proto.UpdateChannelResponse, error) {
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
	return &proto.UpdateChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelAPIFormat(ctx context.Context, req *proto.UpdateChannelAPIFormatRequest) (*proto.UpdateChannelResponse, error) {
	repoChannel, err := s.queries.UpdateChannelAPIFormat(ctx, repository.UpdateChannelAPIFormatParams{
		ID:        req.ChannelId,
		ApiFormat: req.ApiFormat,
	})
	if err != nil {
		zap.L().Error("update channel api format failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.Int32("api_format", req.ApiFormat))
		return nil, err
	}
	zap.L().Info("channel api format updated", zap.Int32("channel_id", repoChannel.ID), zap.Int32("api_format", repoChannel.ApiFormat))
	return &proto.UpdateChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func (s *ChannelService) UpdateChannelProviderKey(ctx context.Context, req *proto.UpdateChannelProviderKeyRequest) (*proto.UpdateChannelResponse, error) {
	repoChannel, err := s.queries.UpdateChannelProviderKey(ctx, repository.UpdateChannelProviderKeyParams{
		ID:          req.ChannelId,
		ProviderKey: req.ProviderKey,
	})
	if err != nil {
		zap.L().Error("update channel provider key failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	zap.L().Info("channel provider key updated", zap.Int32("channel_id", repoChannel.ID))
	return &proto.UpdateChannelResponse{Channel: channelToProto(repoChannel)}, nil
}

func validateChannelName(channelName string) error {
	if strings.TrimSpace(channelName) == "" {
		return fmt.Errorf("channel name is required")
	}
	if len(channelName) > channelNameMaxLength {
		return fmt.Errorf("channel name must be at most %d characters", channelNameMaxLength)
	}
	return nil
}

func validateChannelBaseURL(baseURL string) error {
	if len(baseURL) > channelBaseURLMaxLength {
		return fmt.Errorf("base_url must be at most %d characters", channelBaseURLMaxLength)
	}
	if _, err := coreopenai.ParseBaseURL(baseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}
	return nil
}

func validateChannelStatus(status int32) error {
	switch status {
	case channelStatusActive, channelStatusBanned, channelStatusPending:
		return nil
	default:
		return fmt.Errorf("invalid channel status: %d", status)
	}
}

func channelToProto(channel repository.Channel) *proto.Channel {
	return &proto.Channel{
		ChannelId:   channel.ID,
		ChannelName: channel.ChannelName,
		CreatedAt:   timestamppb.New(channel.CreatedAt),
		Status:      int32(channel.Status),
		BaseUrl:     channel.BaseUrl,
		ApiFormat:   channel.ApiFormat,
	}
}
