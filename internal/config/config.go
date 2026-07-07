package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DB        DBConfig       `mapstructure:"db"`
	Log       LogConfig      `mapstructure:"log"`
	Provider  ProviderConfig `mapstructure:"provider"`
	ProxyAddr string         `mapstructure:"proxyAddr"`
	AdminAddr string         `mapstructure:"adminAddr"`
	LogLevel  string         `mapstructure:"logLevel"`
	WorkDir   string         `mapstructure:"-"`
}

func Load(workDir string) (*Config, error) {
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
	cfg.WorkDir = workDir
	return cfg, nil
}
