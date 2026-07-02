package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"hyper-token/internal/config"
	"hyper-token/logger"

	"github.com/fsnotify/fsnotify"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// get workDir
	path, err := os.Executable()
	if err != nil {
		log.Fatalf("Get working directory error: %v", err)
	}
	workDir := filepath.Dir(path)

	// init viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if runPath, err := os.Getwd(); err == nil {
		viper.AddConfigPath(filepath.Join(runPath, "configs"))
	}
	viper.AddConfigPath(workDir)
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Can not read in config file: %v", err)
	}
	viper.OnConfigChange(func(in fsnotify.Event) {
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Can not read in config file when file content was change: %v", err)
		}
	})
	viper.WatchConfig()

	// init config
	cfg, err := config.Load(workDir)
	if err != nil {
		log.Fatalf("load config file error: %v", err)
	}

	// init zap
	zapLogger := logger.NewLogger(cfg)
	zap.ReplaceGlobals(zapLogger)
	defer zapLogger.Sync()

	// db con str
	dsn := config.GetDSN(cfg.DB)

	// migrate
	if err := runMigrations(dsn); err != nil {
		zap.S().Warnf("migration warning: %v", err)
	}

	// init db engine
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		zap.S().Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		zap.S().Fatalf("failed to ping database: %v", err)
	}
	db.SetMaxIdleConns(cfg.DB.MaxIdle)
	db.SetMaxOpenConns(cfg.DB.MaxOpen)
	db.SetConnMaxLifetime(cfg.DB.MaxLifeTime)
}

func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://db/migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
