package service

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	proto "fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var channelRows = []string{"id", "channel_name", "base_url", "provider_key", "api_format", "status", "created_at"}

func TestChannelServiceCreateAndList(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

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

func TestChannelServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *proto.CreateChannelRequest
	}{
		{
			name: "empty channel name",
			req:  &proto.CreateChannelRequest{ChannelName: "", BaseUrl: "https://api.openai.com", ApiFormat: 1},
		},
		{
			name: "space channel name",
			req:  &proto.CreateChannelRequest{ChannelName: "   ", BaseUrl: "https://api.openai.com", ApiFormat: 1},
		},
		{
			name: "long channel name",
			req:  &proto.CreateChannelRequest{ChannelName: strings.Repeat("a", channelNameMaxLength+1), BaseUrl: "https://api.openai.com", ApiFormat: 1},
		},
		{
			name: "base url with path",
			req:  &proto.CreateChannelRequest{ChannelName: "openai", BaseUrl: "https://api.openai.com/v1", ApiFormat: 1},
		},
		{
			name: "long base url",
			req:  &proto.CreateChannelRequest{ChannelName: "openai", BaseUrl: "https://" + strings.Repeat("a", channelBaseURLMaxLength), ApiFormat: 1},
		},
		{
			name: "missing scheme",
			req:  &proto.CreateChannelRequest{ChannelName: "openai", BaseUrl: "api.openai.com", ApiFormat: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := newServiceMock(t)
			defer cleanup()
			svc := NewChannelService(db)

			if _, err := svc.CreateChannel(context.Background(), tt.req); err == nil {
				t.Fatal("expected validation error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelServiceCreateAllowsEmptyProviderKeyAndAnyAPIFormat(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO channels (channel_name, base_url, provider_key, api_format) VALUES ($1, $2, $3, $4) RETURNING id, channel_name, base_url, provider_key, api_format, status, created_at")).
		WithArgs("custom", "https://api.example.com", "", int32(99)).
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(4), "custom", "https://api.example.com", "", int32(99), int16(1), now))

	created, err := svc.CreateChannel(context.Background(), &proto.CreateChannelRequest{ChannelName: "custom", BaseUrl: "https://api.example.com", ProviderKey: "", ApiFormat: 99})
	if err != nil {
		t.Fatal(err)
	}
	if created.Channel.ApiFormat != 99 {
		t.Fatalf("api_format = %d, want 99", created.Channel.ApiFormat)
	}
}

func TestChannelServiceUpdateChannelName(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

	mock.ExpectQuery("UPDATE channels SET channel_name =").
		WithArgs(int32(1), "renamed").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "renamed", "https://api.openai.com", "provider", int32(1), int16(1), now))

	updated, err := svc.UpdateChannelName(context.Background(), &proto.UpdateChannelNameRequest{ChannelId: 1, ChannelName: "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Channel.ChannelName != "renamed" {
		t.Fatalf("channel_name = %q", updated.Channel.ChannelName)
	}
}

func TestChannelServiceUpdateChannelNameValidation(t *testing.T) {
	tests := []string{"", "   ", strings.Repeat("a", channelNameMaxLength+1)}
	for _, channelName := range tests {
		t.Run(channelName, func(t *testing.T) {
			db, mock, cleanup := newServiceMock(t)
			defer cleanup()
			svc := NewChannelService(db)

			_, err := svc.UpdateChannelName(context.Background(), &proto.UpdateChannelNameRequest{ChannelId: 1, ChannelName: channelName})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelServiceUpdateChannelStatus(t *testing.T) {
	for _, status := range []int32{1, 2, 3} {
		t.Run(strconv.Itoa(int(status)), func(t *testing.T) {
			db, mock, cleanup := newServiceMock(t)
			defer cleanup()
			svc := NewChannelService(db)
			now := time.Now()

			mock.ExpectQuery("UPDATE channels SET status =").
				WithArgs(int32(1), int16(status)).
				WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "openai", "https://api.openai.com", "provider", int32(1), int16(status), now))

			updated, err := svc.UpdateChannelStatus(context.Background(), &proto.UpdateChannelStatusRequest{ChannelId: 1, Status: status})
			if err != nil {
				t.Fatal(err)
			}
			if updated.Channel.Status != status {
				t.Fatalf("status = %d", updated.Channel.Status)
			}
		})
	}
}

func TestChannelServiceUpdateChannelStatusValidation(t *testing.T) {
	for _, status := range []int32{-1, 0, 4} {
		t.Run(strconv.Itoa(int(status)), func(t *testing.T) {
			db, mock, cleanup := newServiceMock(t)
			defer cleanup()
			svc := NewChannelService(db)

			_, err := svc.UpdateChannelStatus(context.Background(), &proto.UpdateChannelStatusRequest{ChannelId: 1, Status: status})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelServiceUpdateChannelBaseURL(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

	mock.ExpectQuery("UPDATE channels SET base_url =").
		WithArgs(int32(1), "https://api.example.com/").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "openai", "https://api.example.com/", "provider", int32(1), int16(1), now))

	updated, err := svc.UpdateChannelBaseURL(context.Background(), &proto.UpdateChannelBaseURLRequest{ChannelId: 1, BaseUrl: "https://api.example.com/"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Channel.BaseUrl != "https://api.example.com/" {
		t.Fatalf("base_url = %q", updated.Channel.BaseUrl)
	}
}

func TestChannelServiceUpdateChannelBaseURLValidation(t *testing.T) {
	tests := []string{"", "api.openai.com", "https://", "https://api.openai.com/v1", "https://" + strings.Repeat("a", channelBaseURLMaxLength)}
	for _, baseURL := range tests {
		t.Run(baseURL, func(t *testing.T) {
			db, mock, cleanup := newServiceMock(t)
			defer cleanup()
			svc := NewChannelService(db)

			_, err := svc.UpdateChannelBaseURL(context.Background(), &proto.UpdateChannelBaseURLRequest{ChannelId: 1, BaseUrl: baseURL})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChannelServiceUpdateChannelAPIFormatAllowsAnyValue(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

	mock.ExpectQuery("UPDATE channels SET api_format =").
		WithArgs(int32(1), int32(99)).
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "openai", "https://api.openai.com", "provider", int32(99), int16(1), now))

	updated, err := svc.UpdateChannelAPIFormat(context.Background(), &proto.UpdateChannelAPIFormatRequest{ChannelId: 1, ApiFormat: 99})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Channel.ApiFormat != 99 {
		t.Fatalf("api_format = %d", updated.Channel.ApiFormat)
	}
}

func TestChannelServiceUpdateChannelProviderKeyAllowsEmptyAndDoesNotExposeValue(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)
	now := time.Now()

	mock.ExpectQuery("UPDATE channels SET provider_key =").
		WithArgs(int32(1), "").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(1), "openai", "https://api.openai.com", "", int32(1), int16(1), now))

	updated, err := svc.UpdateChannelProviderKey(context.Background(), &proto.UpdateChannelProviderKeyRequest{ChannelId: 1, ProviderKey: ""})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Channel.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name("provider_key")) != nil {
		t.Fatal("provider_key is exposed on Channel")
	}
}
