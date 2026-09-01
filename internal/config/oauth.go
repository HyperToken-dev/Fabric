package config

import (
	"fmt"
	"strings"
)

// OAuthConfig controls browser login for the admin server.
// The zero value disables OAuth so local development and unauthenticated legacy
// deployments continue to start unless OAuth is explicitly enabled.
type OAuthConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	IssuerURL     string   `mapstructure:"issuerURL"`
	ClientID      string   `mapstructure:"clientID"`
	ClientSecret  string   `mapstructure:"clientSecret"`
	RedirectURL   string   `mapstructure:"redirectURL"`
	Scopes        []string `mapstructure:"scopes"`
	SessionSecret string   `mapstructure:"sessionSecret"`
}

// Validate rejects incomplete or unsafe OAuth settings before the HTTP server
// starts accepting requests. Empty OAuth configuration is valid and means OAuth
// is disabled.
func (c OAuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.IssuerURL) == "" {
		return fmt.Errorf("oauth.issuerURL is required when oauth is enabled")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("oauth.clientID is required when oauth is enabled")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return fmt.Errorf("oauth.clientSecret is required when oauth is enabled")
	}
	if strings.TrimSpace(c.RedirectURL) == "" {
		return fmt.Errorf("oauth.redirectURL is required when oauth is enabled")
	}
	if len(strings.TrimSpace(c.SessionSecret)) < 32 {
		return fmt.Errorf("oauth.sessionSecret must be at least 32 characters when oauth is enabled")
	}
	return nil
}
