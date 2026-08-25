package service

import (
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	proto "github.com/HyperToken-dev/fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClientChannelServiceReturnsOnlyActiveChannelNames(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewChannelService(db)

	mock.ExpectQuery("SELECT channel_name FROM channels WHERE status = 1").
		WillReturnRows(sqlmock.NewRows([]string{"channel_name"}).AddRow("default"))

	resp, err := svc.ListClientChannels(userTestContext(), &proto.ListClientChannelsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Channels) != 1 || resp.Channels[0].ChannelName != "default" {
		t.Fatalf("client channels = %+v", resp.Channels)
	}
}

func TestClientApiKeyServiceCreatesOwnedKeyByChannelName(t *testing.T) {
	previousReader := secureRandomReader
	secureRandomReader = strings.NewReader(strings.Repeat("b", 32))
	t.Cleanup(func() { secureRandomReader = previousReader })

	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewApiKeyService(db)
	now := time.Now()

	mock.ExpectQuery("SELECT id, channel_name, base_url, provider_key, api_format, status, created_at FROM channels WHERE channel_name").
		WithArgs("default").
		WillReturnRows(sqlmock.NewRows(channelRows).AddRow(int32(7), "default", "https://api.example.com", "provider", int32(1), int16(1), now))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO api_keys (key_hash, key_name, channel_id, owner_openid)\nVALUES ($1, $2, $3, $4)\nRETURNING id, key_hash, key_name, channel_id, owner_openid, created_at")).
		WithArgs(sqlmock.AnyArg(), "client", int32(7), "user-openid").
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_hash", "key_name", "channel_id", "owner_openid", "created_at"}).
			AddRow(int32(1), sql.NullString{String: strings.Repeat("b", 64), Valid: true}, "client", int32(7), "user-openid", now))

	resp, err := svc.CreateClientApiKey(userTestContext(), &proto.CreateClientApiKeyRequest{KeyName: "client", ChannelName: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ApiKey.ChannelName != "default" || resp.ApiKey.RawKey == "" {
		t.Fatalf("client api key = %+v", resp.ApiKey)
	}
}

func TestUsageDashboardScopesUserData(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewUsageService(db, time.UTC)
	svc.now = func() time.Time { return time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC) }

	mock.ExpectQuery("WHERE owner_openid").
		WithArgs("user-openid", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"total_prompt_tokens", "total_completion_tokens", "request_count"}).AddRow(int64(4), int64(6), int64(1)))
	mock.ExpectQuery("WHERE owner_openid").
		WithArgs("UTC", "user-openid", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_prompt_tokens", "total_completion_tokens", "request_count"}))

	resp, err := svc.GetUsageDashboard(userTestContext(), &proto.GetUsageDashboardRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Today.TotalTokens != 10 || resp.Today.RequestCount != 1 {
		t.Fatalf("user dashboard = %+v", resp.Today)
	}
}
