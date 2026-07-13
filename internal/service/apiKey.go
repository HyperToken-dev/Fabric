package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	proto "fabric/gen"
	"fabric/internal/repository"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const keyPrefix = "hy_"

type ApiKeyService struct {
	queries *repository.Queries
}

func NewApiKeyService(db *sql.DB) *ApiKeyService {
	return &ApiKeyService{
		queries: repository.New(db),
	}
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *proto.CreateApiKeyRequest) (res *proto.CreateApiKeyResponse, err error) {
	rawKey, hash := generateApiKey()

	row, err := s.queries.CreateApiKey(ctx, repository.CreateApiKeyParams{
		KeyHash:   sql.NullString{String: hash, Valid: true},
		KeyName:   req.KeyName,
		ChannelID: req.ChannelId,
	})
	if err != nil {
		return nil, err
	}

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
		return nil, err
	}
	return &proto.RevokeApiKeyResponse{}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelID(ctx context.Context, req *proto.ListApiKeysByChannelIDRequest) (res *proto.ListApiKeysResponse, err error) {
	rows, err := s.queries.ListApiKeysByChannelID(ctx, req.ChannelId)
	if err != nil {
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
	return &proto.ListApiKeysResponse{ApiKeys: keys}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelName(ctx context.Context, req *proto.ListApiKeysByChannelNameRequest) (*proto.ListApiKeysResponse, error) {
	rows, err := s.queries.ListApiKeysByChannelName(ctx, req.ChannelName)
	if err != nil {
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
	return &proto.ListApiKeysResponse{ApiKeys: keys}, nil
}

func generateApiKey() (rawKey string, hash string) {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	rawKey = keyPrefix + base64.RawURLEncoding.EncodeToString(bytes)
	h := sha256.Sum256([]byte(rawKey))
	hash = hex.EncodeToString(h[:])
	return
}
