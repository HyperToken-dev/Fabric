package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (rt *Router) ProxyHandler(c *gin.Context) {
	keyIDVal, existsKey := c.Get("keyID")
	channelIDVal, existsChannel := c.Get("channelID")
	baseURLVal, existsBaseURL := c.Get("channelBaseURL")
	providerKeyVal, existsProviderKey := c.Get("channelProviderKey")
	apiFormatVal, existsAPIFormat := c.Get("channelAPIFormat")
	if !existsKey || !existsChannel || !existsBaseURL || !existsProviderKey || !existsAPIFormat {
		zap.L().Warn("proxy context missing", zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID, ok1 := keyIDVal.(int32)
	channelID, ok2 := channelIDVal.(int32)
	baseURL, ok3 := baseURLVal.(string)
	providerKey, ok4 := providerKeyVal.(string)
	apiFormat, ok5 := apiFormatVal.(int32)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		zap.L().Error("proxy context type assertion failed", zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	switch apiFormat {
	case apiFormatOpenAI:
		zap.L().Info("proxy request routed", zap.String("provider", "openai"), zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("api_format", apiFormat))
		rt.openaiProxy.ServeHTTP(c.Writer, c.Request, keyID, channelID, baseURL, providerKey)
	default:
		zap.L().Warn("unsupported api format", zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("api_format", apiFormat))
		c.JSON(http.StatusNotFound, gin.H{"error": "unsupported api format"})
	}
}
