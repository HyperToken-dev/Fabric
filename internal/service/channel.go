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
		channels[i] = &proto.Channel{
			ChannelId:   r.ID,
			ChannelName: r.ChannelName,
			CreatedAt:   timestamppb.New(r.CreatedAt),
			Status:      int32(r.Status),
			BaseUrl:     r.BaseUrl,
			ApiFormat:   r.ApiFormat,
		}
	}
	return &proto.ListChannelsResponse{Channels: channels}, nil
}

func (s *ChannelService) CreateChannel(ctx context.Context, req *proto.CreateChannelRequest) (*proto.CreateChannelResponse, error) {
	repoChannel, err := s.queries.CreateChannel(ctx, repository.CreateChannelParams{
		ChannelName: req.ChannelName,
		BaseUrl:     req.BaseUrl,
		ApiFormat:   req.ApiFormat,
	})
	if err != nil {
		return nil, err
	}
	return &proto.CreateChannelResponse{
		Channel: &proto.Channel{
			ChannelId:   repoChannel.ID,
			ChannelName: repoChannel.ChannelName,
			CreatedAt:   timestamppb.New(repoChannel.CreatedAt),
			Status:      int32(repoChannel.Status),
			BaseUrl:     repoChannel.BaseUrl,
			ApiFormat:   repoChannel.ApiFormat,
		},
	}, nil
}
