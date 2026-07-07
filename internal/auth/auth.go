package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

var bearerPrefix = "Bearer "

func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func ExtractKeyFromRequest(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, bearerPrefix) {
		return "", nil
	}
	return auth[len(bearerPrefix):], nil
}
