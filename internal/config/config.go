package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB        DBConfig       `mapstructure:"db"`
	Log       LogConfig      `mapstructure:"log"`
	OAuth     OAuthConfig    `mapstructure:"oauth"`
	ProxyAddr string         `mapstructure:"proxyAddr"`
	AdminAddr string         `mapstructure:"adminAddr"`
	LogLevel  string         `mapstructure:"logLevel"`
	TimeZone  string         `mapstructure:"timeZone"`
	Location  *time.Location `mapstructure:"-"`
	WorkDir   string         `mapstructure:"-"`
	RunPath   string         `mapstructure:"-"`
}

func Load(workDir, runPath string) (*Config, error) {
	var cfg *Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cfg.ProxyAddr, ":") {
		cfg.ProxyAddr = ":" + cfg.ProxyAddr
	}
	if !strings.HasPrefix(cfg.AdminAddr, ":") {
		cfg.AdminAddr = ":" + cfg.AdminAddr
	}
	cfg.TimeZone = strings.TrimSpace(cfg.TimeZone)
	if cfg.TimeZone == "" {
		cfg.TimeZone = "UTC"
	}
	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("invalid TimeZone %q: %w", cfg.TimeZone, err)
	}
	cfg.Location = location
	cfg.WorkDir, cfg.RunPath = workDir, runPath
	if err := cfg.OAuth.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// OAuthConfig controls browser login for the admin server.
//
// The zero value disables OAuth so local development and unauthenticated legacy
// deployments continue to start unless OAuth is explicitly enabled.
type OAuthConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	IssuerURL      string   `mapstructure:"issuerURL"`
	ClientID       string   `mapstructure:"clientID"`
	ClientSecret   string   `mapstructure:"clientSecret"`
	RedirectURL    string   `mapstructure:"redirectURL"`
	Scopes         []string `mapstructure:"scopes"`
	SessionSecret  string   `mapstructure:"sessionSecret"`
	AllowedDomains []string `mapstructure:"allowedDomains"`
	AdminEmails    []string `mapstructure:"adminEmails"`
	AutoProvision  bool     `mapstructure:"autoProvision"`
	DefaultRole    string   `mapstructure:"defaultRole"`
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
	role := strings.TrimSpace(c.DefaultRole)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		return fmt.Errorf("oauth.defaultRole must be user or admin")
	}
	return nil
}
