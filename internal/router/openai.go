package router

import (
	"hyper-token/internal/proxy"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (rt *Router) OpenAIHandler(c *gin.Context) {
	keyIDVal, existsKey := c.Get("keyID")
	channelIDVal, existsChannel := c.Get("channelID")
	if !existsKey || !existsChannel {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID, ok1 := keyIDVal.(int32)
	channelID, ok2 := channelIDVal.(int32)
	if !ok1 || !ok2 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	provider := proxy.Route(c.Request)
	switch provider {
	case proxy.ProviderOpenAI:
		rt.openaiProxy.ServeHTTP(c.Writer, c.Request, keyID, channelID)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unsupported provider"})
	}
}
