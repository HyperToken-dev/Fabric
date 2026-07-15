package config

type LogConfig struct {
	MaxSize    int  `mapstructure:"maxSize"`
	MaxBackups int  `mapstructure:"maxBackups"`
	MaxAge     int  `mapstructure:"maxAge"`
	Compress   bool `mapstructure:"compress"`
}
