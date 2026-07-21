package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB                    DBConfig                    `mapstructure:"db"`
	Log                   LogConfig                   `mapstructure:"log"`
	SensitiveWD           bool                        `mapstructure:"sensitiveWordDetect"`
	SensitiveDictionaries []SensitiveDictionaryConfig `mapstructure:"sensitiveWordDictionaries"`
	ProxyAddr             string                      `mapstructure:"proxyAddr"`
	AdminAddr             string                      `mapstructure:"adminAddr"`
	LogLevel              string                      `mapstructure:"logLevel"`
	TimeZone              string                      `mapstructure:"timeZone"`
	Location              *time.Location              `mapstructure:"-"`
	WorkDir               string                      `mapstructure:"-"`
	RunPath               string                      `mapstructure:"-"`
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
	return cfg, nil
}
