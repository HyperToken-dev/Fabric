package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	"github.com/HyperToken-dev/fabric/internal/service"
)

type gatewaySensitiveSource struct {
	runtimeStore *service.SensitiveRuntimeStore
}

func (s gatewaySensitiveSource) State(ctx context.Context) (sensitive.SourceState, error) {
	if s.runtimeStore == nil {
		return sensitive.SourceState{Enabled: false}, nil
	}
	state, err := s.runtimeStore.ReadState(ctx)
	if err != nil {
		return sensitive.SourceState{}, err
	}
	if !state.Enabled {
		return sensitive.SourceState{Enabled: false}, nil
	}

	dictionaries, err := s.runtimeStore.ListDictionaries(ctx)
	if err != nil {
		return sensitive.SourceState{}, err
	}
	loaded := make([]sensitive.Dictionary, 0, len(dictionaries))
	for _, dict := range dictionaries {
		if !dict.Enabled || len(dict.Words) == 0 {
			continue
		}
		loaded = append(loaded, sensitive.Dictionary{
			Name:         dict.Name,
			Words:        append([]string(nil), dict.Words...),
			EffectModels: append([]string(nil), dict.EffectModels...),
		})
	}
	detector, err := sensitive.NewDetector(loaded...)
	if err != nil {
		return sensitive.SourceState{}, err
	}
	return sensitive.SourceState{Enabled: true, Detector: detector, DictionaryCount: len(loaded)}, nil
}

func sensitiveRuntimePath(workDir, runPath string) string {
	path := filepath.Join(workDir, "sensitive")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Join(runPath, "configs", "sensitive")
}
