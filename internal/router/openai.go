package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (rt *Router) ProxyHandler(c *gin.Context) {
	keyIDVal, existsKey := c.Get("keyID")
	channelIDVal, existsChannel := c.Get("channelID")
	baseURLVal, existsBaseURL := c.Get("channelBaseURL")
	apiFormatVal, existsAPIFormat := c.Get("channelAPIFormat")
	if !existsKey || !existsChannel || !existsBaseURL || !existsAPIFormat {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID, ok1 := keyIDVal.(int32)
	channelID, ok2 := channelIDVal.(int32)
	baseURL, ok3 := baseURLVal.(string)
	apiFormat, ok4 := apiFormatVal.(int32)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	switch apiFormat {
	case apiFormatOpenAI:
		rt.openaiProxy.ServeHTTP(c.Writer, c.Request, keyID, channelID, baseURL)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unsupported api format"})
	}
}
