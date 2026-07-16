package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"io"

	proto "github.com/HyperToken-dev/fabric/gen"
	"github.com/HyperToken-dev/fabric/internal/repository"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const keyPrefix = "hy_"

var secureRandomReader io.Reader = rand.Reader

type ApiKeyService struct {
	queries *repository.Queries
}

func NewApiKeyService(db *sql.DB) *ApiKeyService {
	return &ApiKeyService{
		queries: repository.New(db),
	}
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *proto.CreateApiKeyRequest) (res *proto.CreateApiKeyResponse, err error) {
	rawKey, hash, err := generateApiKey()
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateApiKey(ctx, repository.CreateApiKeyParams{
		KeyHash:   sql.NullString{String: hash, Valid: true},
		KeyName:   req.KeyName,
		ChannelID: req.ChannelId,
	})
	if err != nil {
		zap.L().Error("create api key failed", zap.Error(err), zap.String("key_name", req.KeyName), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	zap.L().Info("api key created",
		zap.String("key_name", row.KeyName),
		zap.Int32("channel_id", row.ChannelID),
		zap.String("key_hash_prefix", keyHashPrefix(row.KeyHash.String)),
	)

	return &proto.CreateApiKeyResponse{
		ApiKey: &proto.ApiKey{
			KeyHash:   row.KeyHash.String,
			RawKey:    rawKey, // return once only when create
			KeyName:   row.KeyName,
			CreatedAt: timestamppb.New(row.CreatedAt),
		},
	}, nil
}

func (s *ApiKeyService) RevokeApiKey(ctx context.Context, req *proto.RevokeApiKeyRequest) (*proto.RevokeApiKeyResponse, error) {
	err := s.queries.DeleteApiKeyByHash(ctx, sql.NullString{String: req.KeyHash, Valid: true})
	if err != nil {
		zap.L().Error("revoke api key failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
		return nil, err
	}
	zap.L().Info("api key revoked", zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
	return &proto.RevokeApiKeyResponse{}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelID(ctx context.Context, req *proto.ListApiKeysByChannelIDRequest) (res *proto.ListApiKeysResponse, err error) {
	rows, err := s.queries.ListApiKeysByChannelID(ctx, req.ChannelId)
	if err != nil {
		zap.L().Error("list api keys by channel id failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	keys := make([]*proto.ApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.ApiKey{
			KeyHash:   r.KeyHash.String,
			KeyName:   r.KeyName,
			CreatedAt: timestamppb.New(r.CreatedAt),
		}
	}
	zap.L().Info("api keys listed by channel id", zap.Int32("channel_id", req.ChannelId), zap.Int("count", len(keys)))
	return &proto.ListApiKeysResponse{ApiKeys: keys}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelName(ctx context.Context, req *proto.ListApiKeysByChannelNameRequest) (*proto.ListApiKeysResponse, error) {
	rows, err := s.queries.ListApiKeysByChannelName(ctx, req.ChannelName)
	if err != nil {
		zap.L().Error("list api keys by channel name failed", zap.Error(err), zap.String("channel_name", req.ChannelName))
		return nil, err
	}
	keys := make([]*proto.ApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.ApiKey{
			KeyHash:   r.KeyHash.String,
			KeyName:   r.KeyName,
			CreatedAt: timestamppb.New(r.CreatedAt),
		}
	}
	zap.L().Info("api keys listed by channel name", zap.String("channel_name", req.ChannelName), zap.Int("count", len(keys)))
	return &proto.ListApiKeysResponse{ApiKeys: keys}, nil
}

func generateApiKey() (rawKey string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(secureRandomReader, bytes); err != nil {
		return "", "", err
	}
	rawKey = keyPrefix + base64.RawURLEncoding.EncodeToString(bytes)
	h := sha256.Sum256([]byte(rawKey))
	hash = hex.EncodeToString(h[:])
	return rawKey, hash, nil
}
