package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	"github.com/HyperToken-dev/fabric/internal/config"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type gatewaySensitiveSource struct {
	workDir string
	runPath string
}

func (s gatewaySensitiveSource) State(ctx context.Context) (sensitive.SourceState, error) {
	if err := ctx.Err(); err != nil {
		return sensitive.SourceState{}, err
	}
	if err := viper.ReadInConfig(); err != nil {
		return sensitive.SourceState{}, fmt.Errorf("read sensitive config: %w", err)
	}
	cfg, err := config.Load(s.workDir, s.runPath)
	if err != nil {
		return sensitive.SourceState{}, fmt.Errorf("load sensitive config: %w", err)
	}
	if !cfg.SensitiveWD {
		return sensitive.SourceState{Enabled: false}, nil
	}

	dictionaries := make([]sensitive.DictionaryFileConfig, 0, len(cfg.SensitiveDictionaries))
	for _, dictConfig := range cfg.SensitiveDictionaries {
		dictionaries = append(dictionaries, sensitive.DictionaryFileConfig{
			Name:            dictConfig.Name,
			EffectModels:    append([]string(nil), dictConfig.EffectModels...),
			KeywordFileList: append([]string(nil), dictConfig.KeywordFileList...),
		})
	}
	detector, err := sensitive.LoadDetectorFromFiles(sensitiveWordsPath(cfg.WorkDir, cfg.RunPath), dictionaries)
	if err != nil {
		return sensitive.SourceState{}, err
	}
	return sensitive.SourceState{Enabled: true, Detector: detector, DictionaryCount: len(cfg.SensitiveDictionaries)}, nil
}

func (s gatewaySensitiveSource) Watch(ctx context.Context, policy *sensitive.ReloadablePolicy, cfg *config.Config) {
	paths := []string{viper.ConfigFileUsed(), sensitiveWordsPath(cfg.WorkDir, cfg.RunPath)}
	go func() {
		err := sensitive.Watch(ctx, sensitive.WatchOptions{
			Paths: paths,
			Reload: func(ctx context.Context) error {
				snapshot, err := policy.Reload(ctx, s.State)
				if err != nil {
					zap.L().Warn("sensitive dictionaries reload failed", zap.Error(err))
					return nil
				}
				zap.L().Info("sensitive dictionaries reloaded",
					zap.Bool("enabled", snapshot.Enabled),
					zap.Int64("version", snapshot.Version),
					zap.Time("loaded_at", snapshot.LoadedAt),
					zap.Int("count", snapshot.DictionaryCount),
				)
				return nil
			},
		})
		if err != nil && err != context.Canceled {
			zap.L().Warn("sensitive dictionary watcher stopped", zap.Error(err))
		}
	}()
}

func sensitiveWordsPath(workDir, runPath string) string {
	path := filepath.Join(workDir, "stwd")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Join(runPath, "configs", "stwd")
}
