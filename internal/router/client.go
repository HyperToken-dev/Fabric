package router

import (
	"database/sql"
	"fabric/internal/auth"
	"fabric/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Router struct {
	queries     *repository.Queries
	openaiProxy OpenAIProxy
}

type OpenAIProxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string)
}

const (
	channelStatusEnabled int16 = 1
	apiFormatOpenAI      int32 = 1
)

func New(queries *repository.Queries, openaiProxy OpenAIProxy) *Router {
	return &Router{queries: queries, openaiProxy: openaiProxy}
}

func (rt *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey, err := auth.ExtractKeyFromRequest(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		keyHash := auth.HashKey(rawKey)
		hash := sql.NullString{String: keyHash, Valid: true}
		row, err := rt.queries.GetApiKeyWithChannelByHash(c.Request.Context(), hash)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			c.Abort()
			return
		}
		if row.ChannelStatus != channelStatusEnabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "channel disabled"})
			c.Abort()
			return
		}

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
