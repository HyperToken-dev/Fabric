package service

import (
	"context"
	"database/sql"

	proto "hyper-token/gen"
	"hyper-token/internal/repository"

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
		return nil, err
	}

	channels := make([]*proto.Channel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
	return &proto.ListChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) ListActiveChannels(ctx context.Context, req *proto.ListActiveChannelsRequest) (*proto.ListChannelsResponse, error) {
	rows, err := s.queries.ListActiveChannels(ctx)
	if err != nil {
		return nil, err
	}

	channels := make([]*proto.Channel, len(rows))
	for i, r := range rows {
		channels[i] = channelToProto(r)
	}
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
		return nil, err
	}
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
