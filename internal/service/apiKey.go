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
	"github.com/HyperToken-dev/fabric/internal/adminauth"
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

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *proto.CreateAdminApiKeyRequest) (res *proto.CreateAdminApiKeyResponse, err error) {
	user, err := adminauth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	channel, err := s.queries.GetChannelById(ctx, req.ChannelId)
	if err == sql.ErrNoRows {
		return nil, ValidationError{Message: "channel is required"}
	}
	if err != nil {
		zap.L().Error("create admin api key channel lookup failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId), zap.String("owner_openid", user.OpenID))
		return nil, err
	}
	return s.createApiKeyForChannel(ctx, user.OpenID, req.KeyName, channel.ID, channel.ChannelName)
}

func (s *ApiKeyService) CreateClientApiKey(ctx context.Context, req *proto.CreateClientApiKeyRequest) (*proto.CreateClientApiKeyResponse, error) {
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	channel, err := s.queries.GetActiveChannelByName(ctx, req.ChannelName)
	if err == sql.ErrNoRows {
		zap.L().Warn("create client api key channel lookup failed", zap.Error(err), zap.String("channel_name", req.ChannelName), zap.String("owner_openid", user.OpenID))
		return nil, ValidationError{Message: "active channel is required"}
	}
	if err != nil {
		zap.L().Error("create client api key channel lookup failed", zap.Error(err), zap.String("channel_name", req.ChannelName), zap.String("owner_openid", user.OpenID))
		return nil, err
	}
	res, err := s.createApiKeyForChannel(ctx, user.OpenID, req.KeyName, channel.ID, channel.ChannelName)
	if err != nil {
		return nil, err
	}
	return &proto.CreateClientApiKeyResponse{ApiKey: &proto.ClientApiKey{
		KeyId:       res.ApiKey.KeyId,
		KeyHash:     res.ApiKey.KeyHash,
		RawKey:      res.ApiKey.RawKey,
		KeyName:     res.ApiKey.KeyName,
		ChannelName: channel.ChannelName,
		OwnerOpenid: user.OpenID,
		CreatedAt:   res.ApiKey.CreatedAt,
	}}, nil
}

func (s *ApiKeyService) createApiKeyForChannel(ctx context.Context, ownerOpenID string, keyName string, channelID int32, channelName string) (res *proto.CreateAdminApiKeyResponse, err error) {
	rawKey, hash, err := generateApiKey()
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateApiKey(ctx, repository.CreateApiKeyParams{
		KeyHash:     sql.NullString{String: hash, Valid: true},
		KeyName:     keyName,
		ChannelID:   channelID,
		OwnerOpenid: ownerOpenID,
	})
	if err != nil {
		zap.L().Error("create api key failed", zap.Error(err), zap.String("key_name", keyName), zap.Int32("channel_id", channelID), zap.String("owner_openid", ownerOpenID))
		return nil, err
	}
	zap.L().Info("api key created",
		zap.String("key_name", row.KeyName),
		zap.Int32("channel_id", row.ChannelID),
		zap.String("owner_openid", row.OwnerOpenid),
		zap.String("channel_name", channelName),
		zap.String("key_hash_prefix", keyHashPrefix(row.KeyHash.String)),
	)

	return &proto.CreateAdminApiKeyResponse{
		ApiKey: &proto.AdminApiKey{
			KeyId:       row.ID,
			ChannelId:   row.ChannelID,
			ChannelName: channelName,
			KeyHash:     row.KeyHash.String,
			RawKey:      rawKey, // return once only when create
			KeyName:     row.KeyName,
			OwnerOpenid: row.OwnerOpenid,
			CreatedAt:   timestamppb.New(row.CreatedAt),
		},
	}, nil
}

func (s *ApiKeyService) RevokeApiKey(ctx context.Context, req *proto.RevokeAdminApiKeyRequest) (*proto.RevokeAdminApiKeyResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	err := s.queries.DeleteApiKeyByHash(ctx, sql.NullString{String: req.KeyHash, Valid: true})
	if err != nil {
		zap.L().Error("revoke api key failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
		return nil, err
	}
	zap.L().Info("api key revoked", zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)))
	return &proto.RevokeAdminApiKeyResponse{}, nil
}

func (s *ApiKeyService) RevokeClientApiKey(ctx context.Context, req *proto.RevokeClientApiKeyRequest) (*proto.RevokeClientApiKeyResponse, error) {
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	err = s.queries.DeleteApiKeyByHashAndOwnerOpenID(ctx, repository.DeleteApiKeyByHashAndOwnerOpenIDParams{
		KeyHash:     sql.NullString{String: req.KeyHash, Valid: true},
		OwnerOpenid: user.OpenID,
	})
	if err != nil {
		zap.L().Error("revoke client api key failed", zap.Error(err), zap.String("key_hash_prefix", keyHashPrefix(req.KeyHash)), zap.String("owner_openid", user.OpenID))
		return nil, err
	}
	return &proto.RevokeClientApiKeyResponse{}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelID(ctx context.Context, req *proto.ListAdminApiKeysByChannelIDRequest) (res *proto.ListAdminApiKeysResponse, err error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	channel, err := s.queries.GetChannelById(ctx, req.ChannelId)
	if err == sql.ErrNoRows {
		return nil, ValidationError{Message: "channel is required"}
	}
	if err != nil {
		zap.L().Error("list api keys channel lookup failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	rows, err := s.queries.ListApiKeysByChannelID(ctx, req.ChannelId)
	if err != nil {
		zap.L().Error("list api keys by channel id failed", zap.Error(err), zap.Int32("channel_id", req.ChannelId))
		return nil, err
	}
	keys := make([]*proto.AdminApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.AdminApiKey{
			KeyId:       r.ID,
			ChannelId:   r.ChannelID,
			ChannelName: channel.ChannelName,
			KeyHash:     r.KeyHash.String,
			KeyName:     r.KeyName,
			OwnerOpenid: r.OwnerOpenid,
			CreatedAt:   timestamppb.New(r.CreatedAt),
		}
	}
	zap.L().Info("api keys listed by channel id", zap.Int32("channel_id", req.ChannelId), zap.Int("count", len(keys)))
	return &proto.ListAdminApiKeysResponse{ApiKeys: keys}, nil
}

func (s *ApiKeyService) ListApiKeys(ctx context.Context, req *proto.ListAdminApiKeysRequest) (*proto.ListAdminApiKeysResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListApiKeysWithChannel(ctx)
	if err != nil {
		zap.L().Error("list api keys failed", zap.Error(err))
		return nil, err
	}
	keys := make([]*proto.AdminApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.AdminApiKey{
			KeyId:       r.ID,
			ChannelId:   r.ChannelID,
			ChannelName: r.ChannelName,
			KeyHash:     r.KeyHash.String,
			KeyName:     r.KeyName,
			OwnerOpenid: r.OwnerOpenid,
			CreatedAt:   timestamppb.New(r.CreatedAt),
		}
	}
	zap.L().Info("api keys listed", zap.Int("count", len(keys)))
	return &proto.ListAdminApiKeysResponse{ApiKeys: keys}, nil
}

func (s *ApiKeyService) ListApiKeysByChannelName(ctx context.Context, req *proto.ListAdminApiKeysByChannelNameRequest) (*proto.ListAdminApiKeysResponse, error) {
	if _, err := adminauth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListApiKeysByChannelName(ctx, req.ChannelName)
	if err != nil {
		zap.L().Error("list api keys by channel name failed", zap.Error(err), zap.String("channel_name", req.ChannelName))
		return nil, err
	}
	keys := make([]*proto.AdminApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.AdminApiKey{
			KeyId:       r.ID,
			ChannelId:   r.ChannelID,
			ChannelName: req.ChannelName,
			KeyHash:     r.KeyHash.String,
			KeyName:     r.KeyName,
			OwnerOpenid: r.OwnerOpenid,
			CreatedAt:   timestamppb.New(r.CreatedAt),
		}
	}
	zap.L().Info("api keys listed by channel name", zap.String("channel_name", req.ChannelName), zap.Int("count", len(keys)))
	return &proto.ListAdminApiKeysResponse{ApiKeys: keys}, nil
}

func (s *ApiKeyService) ListClientApiKeys(ctx context.Context, req *proto.ListClientApiKeysRequest) (*proto.ListClientApiKeysResponse, error) {
	user, err := adminauth.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListApiKeysWithChannelByOwnerOpenID(ctx, user.OpenID)
	if err != nil {
		zap.L().Error("list client api keys failed", zap.Error(err), zap.String("owner_openid", user.OpenID))
		return nil, err
	}
	keys := make([]*proto.ClientApiKey, len(rows))
	for i, r := range rows {
		keys[i] = &proto.ClientApiKey{
			KeyId:       r.ID,
			KeyHash:     r.KeyHash.String,
			KeyName:     r.KeyName,
			ChannelName: r.ChannelName,
			OwnerOpenid: r.OwnerOpenid,
			CreatedAt:   timestamppb.New(r.CreatedAt),
		}
	}
	return &proto.ListClientApiKeysResponse{ApiKeys: keys}, nil
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
