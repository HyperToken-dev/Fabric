package router

import (
	"database/sql"
	"github.com/HyperToken-dev/fabric/internal/auth"
	"github.com/HyperToken-dev/fabric/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Router struct {
	queries *repository.Queries
	proxies map[int32]Proxy
}

type Proxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string)
}

const (
	channelStatusEnabled int16 = 1
)

func New(queries *repository.Queries, proxies map[int32]Proxy) *Router {
	return &Router{queries: queries, proxies: proxies}
}

func (rt *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey, err := auth.ExtractKeyFromRequest(c.Request)
		if err != nil {
			zap.L().Warn("proxy authentication failed",
				zap.String("reason", "missing_or_invalid_authorization"),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.Request.RemoteAddr),
				zap.Error(err),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		keyHash := auth.HashKey(rawKey)
		hash := sql.NullString{String: keyHash, Valid: true}
		row, err := rt.queries.GetApiKeyWithChannelByHash(c.Request.Context(), hash)
		if err != nil {
			zap.L().Warn("proxy authentication failed",
				zap.String("reason", "invalid_api_key"),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.Request.RemoteAddr),
				zap.String("key_hash_prefix", keyHashPrefix(keyHash)),
				zap.Error(err),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			c.Abort()
			return
		}
		if row.ChannelStatus != channelStatusEnabled {
			zap.L().Warn("proxy authentication failed",
				zap.String("reason", "channel_disabled"),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.Request.RemoteAddr),
				zap.Int32("key_id", row.KeyID),
				zap.Int32("channel_id", row.ChannelID),
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "channel disabled"})
			c.Abort()
			return
		}

		zap.L().Info("proxy authentication succeeded",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.Int32("key_id", row.KeyID),
			zap.Int32("channel_id", row.ChannelID),
			zap.Int32("api_format", row.ChannelApiFormat),
		)

		// Store in context for handlers
		c.Set("keyID", row.KeyID)
		c.Set("channelID", row.ChannelID)
		c.Set("channelBaseURL", row.BaseUrl)
		c.Set("channelProviderKey", row.ProviderKey)
		c.Set("channelAPIFormat", row.ChannelApiFormat)
		c.Next()
	}
}

func (rt *Router) RegisterProxyRoutes() *gin.Engine {
	// Set gin to release mode for production performance
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery()) // Standard recovery middleware

	// Global Catch-all with Auth
	r.Use(rt.AuthMiddleware())
	r.Any("/*any", rt.ProxyHandler)

	return r
}
