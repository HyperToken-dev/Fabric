package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	proto "fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestChannelServiceCreateAndList(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()
	channelRows := []string{"id", "channel_name", "base_url", "provider_key", "api_format", "status", "created_at"}

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO channels (channel_name, base_url, provider_key, api_format) VALUES ($1, $2, $3, $4) RETURNING id, channel_name, base_url, provider_key, api_format, status, created_at")).
		WithArgs("openai", "https://api.openai.com", "provider", int32(1)).
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "openai", "https://api.openai.com", "provider", int32(1), int16(1), now))
	created, err := svc.CreateChannel(context.Background(), &proto.CreateChannelRequest{ChannelName: "openai", BaseUrl: "https://api.openai.com", ProviderKey: "provider", ApiFormat: 1})
	if err != nil {
		t.Fatal(err)
	}
	if created.Channel.ChannelId != 1 || created.Channel.ChannelName != "openai" {
		t.Fatalf("created channel = %+v", created.Channel)
	}

	mock.ExpectQuery("SELECT id, channel_name, base_url, provider_key, api_format, status, created_at FROM channels ORDER BY").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(2), "other", "http://base", "key", int32(1), int16(1), now))
	listed, err := svc.ListChannels(context.Background(), &proto.ListChannelsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Channels) != 1 || listed.Channels[0].ChannelName != "other" {
		t.Fatalf("listed channels = %+v", listed.Channels)
	}

	mock.ExpectQuery("SELECT id, channel_name, base_url, provider_key, api_format, status, created_at FROM channels WHERE status = 1").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(3), "active", "http://base", "key", int32(1), int16(1), now))
	active, err := svc.ListActiveChannels(context.Background(), &proto.ListActiveChannelsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(active.Channels) != 1 || active.Channels[0].ChannelName != "active" {
		t.Fatalf("active channels = %+v", active.Channels)
	}
}
