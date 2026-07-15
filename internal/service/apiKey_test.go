package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	proto "fabric/gen"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApiKeyServiceCreateApiKey(t *testing.T) {
	previousReader := secureRandomReader
	secureRandomReader = strings.NewReader(strings.Repeat("a", 32))
	t.Cleanup(func() { secureRandomReader = previousReader })

	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewApiKeyService(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO api_keys (key_hash, key_name, channel_id) VALUES ($1, $2, $3) RETURNING id, key_hash, key_name, channel_id, created_at")).
		WithArgs(sqlmock.AnyArg(), "primary", int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_hash", "key_name", "channel_id", "created_at"}).
			AddRow(int32(1), sql.NullString{String: strings.Repeat("a", 64), Valid: true}, "primary", int32(7), now))

	res, err := svc.CreateApiKey(context.Background(), &proto.CreateApiKeyRequest{KeyName: "primary", ChannelId: 7})
	if err != nil {
		t.Fatal(err)
	}
	if res.ApiKey.RawKey == "" || !strings.HasPrefix(res.ApiKey.RawKey, keyPrefix) {
		t.Fatalf("RawKey = %q, want prefix %q", res.ApiKey.RawKey, keyPrefix)
	}
	if res.ApiKey.KeyName != "primary" || res.ApiKey.KeyHash != strings.Repeat("a", 64) {
		t.Fatalf("ApiKey = %+v", res.ApiKey)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApiKeyServiceCreateApiKeyRandomFailure(t *testing.T) {
	previousReader := secureRandomReader
	wantErr := errors.New("random failed")
	secureRandomReader = failingReader{err: wantErr}
	t.Cleanup(func() { secureRandomReader = previousReader })

	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewApiKeyService(db)

	_, err := svc.CreateApiKey(context.Background(), &proto.CreateApiKeyRequest{KeyName: "primary", ChannelId: 7})
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateApiKey() error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApiKeyServiceRevokeAndList(t *testing.T) {
	db, mock, cleanup := newServiceMock(t)
	defer cleanup()
	svc := NewApiKeyService(db)
	now := time.Now()

	mock.ExpectExec("DELETE FROM api_keys WHERE key_hash").
		WithArgs(sql.NullString{String: "hash", Valid: true}).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := svc.RevokeApiKey(context.Background(), &proto.RevokeApiKeyRequest{KeyHash: "hash"}); err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, key_hash, key_name, channel_id, created_at FROM api_keys").
		WithArgs(int32(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_hash", "key_name", "channel_id", "created_at"}).
			AddRow(int32(1), sql.NullString{String: "hash", Valid: true}, "primary", int32(7), now))
	list, err := svc.ListApiKeysByChannelID(context.Background(), &proto.ListApiKeysByChannelIDRequest{ChannelId: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.ApiKeys) != 1 || list.ApiKeys[0].RawKey != "" || list.ApiKeys[0].KeyName != "primary" {
		t.Fatalf("ApiKeys = %+v", list.ApiKeys)
	}

	mock.ExpectQuery("JOIN channels ON api_keys.channel_id = channels.id").
		WithArgs("openai").
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_hash", "key_name", "channel_id", "created_at"}).
			AddRow(int32(2), sql.NullString{String: "hash2", Valid: true}, "secondary", int32(7), now))
	byName, err := svc.ListApiKeysByChannelName(context.Background(), &proto.ListApiKeysByChannelNameRequest{ChannelName: "openai"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byName.ApiKeys) != 1 || byName.ApiKeys[0].KeyName != "secondary" {
		t.Fatalf("ApiKeys by name = %+v", byName.ApiKeys)
	}
}

type failingReader struct {
	err error
}

func (r failingReader) Read(p []byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = failingReader{}
