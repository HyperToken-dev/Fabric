package service

import (
	"context"
	"database/sql"

	proto "fabric/gen"
	"fabric/internal/repository"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChannelService struct {
	queries *repository.Queries
}

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
