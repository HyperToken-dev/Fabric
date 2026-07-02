package config

import (
	"fmt"
	"time"
)

type DBConfig struct {
	User        string        `mapstructure:"user"`
	Port        string        `mapstructure:"port"`
	Password    string        `mapstructure:"password"`
	DBName      string        `mapstructure:"dbName"`
	Addr        string        `mapstructure:"addr"`
	MaxIdle     int           `mapstructure:"maxIdle"`
	MaxOpen     int           `mapstructure:"maxOpen"`
	MaxLifeTime time.Duration `mapstructure:"maxLifeTime"`
}

func GetDSN(dbConfig DBConfig) string {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbConfig.User, dbConfig.Password, dbConfig.Addr, dbConfig.Port, dbConfig.DBName)
	return dsn
}
