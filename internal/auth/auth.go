package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

var bearerPrefix = "Bearer "

var ErrMissingBearerToken = errors.New("missing bearer token")

func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func ExtractKeyFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", ErrMissingBearerToken
	}
	key := strings.TrimSpace(authHeader[len(bearerPrefix):])
	if key == "" {
		return "", ErrMissingBearerToken
	}
	return key, nil
}
