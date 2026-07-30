package service

import (
	"context"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	proto "github.com/HyperToken-dev/fabric/gen"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SensitiveWordService struct {
	store  *SensitiveRuntimeStore
	policy *sensitive.ReloadablePolicy
	source sensitive.SourceFunc
}

func NewSensitiveWordService(store *SensitiveRuntimeStore, policy *sensitive.ReloadablePolicy, source sensitive.SourceFunc) *SensitiveWordService {
	return &SensitiveWordService{store: store, policy: policy, source: source}
}

func (s *SensitiveWordService) GetSensitiveWordStatus(ctx context.Context, req *proto.GetSensitiveWordStatusRequest) (*proto.GetSensitiveWordStatusResponse, error) {
	state, err := s.store.ReadState(ctx)
	if err != nil {
		zap.L().Error("read sensitive word status failed", zap.Error(err))
		return nil, err
	}
	return &proto.GetSensitiveWordStatusResponse{
		Enabled:          state.Enabled,
		StoreInitialized: true,
		Snapshot:         sensitiveSnapshotToProto(s.policy.Snapshot()),
	}, nil
}

func (s *SensitiveWordService) UpdateSensitiveWordEnabled(ctx context.Context, req *proto.UpdateSensitiveWordEnabledRequest) (*proto.GetSensitiveWordStatusResponse, error) {
	if err := s.store.SetEnabled(ctx, req.Enabled); err != nil {
		zap.L().Error("update sensitive word enabled failed", zap.Error(err), zap.Bool("enabled", req.Enabled))
		return nil, err
	}
	snapshot, err := s.reload(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.GetSensitiveWordStatusResponse{Enabled: req.Enabled, StoreInitialized: true, Snapshot: sensitiveSnapshotToProto(snapshot)}, nil
}

func (s *SensitiveWordService) ListSensitiveDictionaries(ctx context.Context, req *proto.ListSensitiveDictionariesRequest) (*proto.ListSensitiveDictionariesResponse, error) {
	dicts, err := s.store.ListDictionaries(ctx)
	if err != nil {
		zap.L().Error("list sensitive dictionaries failed", zap.Error(err))
		return nil, err
	}
	items := make([]*proto.SensitiveDictionarySummary, 0, len(dicts))
	for _, dict := range dicts {
		items = append(items, sensitiveDictionarySummaryToProto(dict))
	}
	return &proto.ListSensitiveDictionariesResponse{Dictionaries: items}, nil
}

func (s *SensitiveWordService) GetSensitiveDictionary(ctx context.Context, req *proto.GetSensitiveDictionaryRequest) (*proto.GetSensitiveDictionaryResponse, error) {
	dict, err := s.store.GetDictionary(ctx, req.Name)
	if err != nil {
		zap.L().Error("get sensitive dictionary failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	return &proto.GetSensitiveDictionaryResponse{Dictionary: sensitiveDictionaryToProto(dict)}, nil
}

func (s *SensitiveWordService) CreateSensitiveDictionary(ctx context.Context, req *proto.CreateSensitiveDictionaryRequest) (*proto.SensitiveDictionaryResponse, error) {
	dict, err := s.store.CreateDictionary(ctx, req.Name, req.EffectModels, req.Enabled, req.Words)
	if err != nil {
		zap.L().Error("create sensitive dictionary failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	return s.dictionaryResponse(ctx, dict)
}

func (s *SensitiveWordService) UpdateSensitiveDictionaryEffectModels(ctx context.Context, req *proto.UpdateSensitiveDictionaryEffectModelsRequest) (*proto.SensitiveDictionaryResponse, error) {
	dict, err := s.store.UpdateEffectModels(ctx, req.Name, req.EffectModels)
	if err != nil {
		zap.L().Error("update sensitive dictionary effect models failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	return s.dictionaryResponse(ctx, dict)
}

func (s *SensitiveWordService) UpdateSensitiveDictionaryEnabled(ctx context.Context, req *proto.UpdateSensitiveDictionaryEnabledRequest) (*proto.SensitiveDictionaryResponse, error) {
	dict, err := s.store.UpdateDictionaryEnabled(ctx, req.Name, req.Enabled)
	if err != nil {
		zap.L().Error("update sensitive dictionary enabled failed", zap.Error(err), zap.String("name", req.Name), zap.Bool("enabled", req.Enabled))
		return nil, err
	}
	return s.dictionaryResponse(ctx, dict)
}

func (s *SensitiveWordService) AddSensitiveWords(ctx context.Context, req *proto.AddSensitiveWordsRequest) (*proto.SensitiveDictionaryResponse, error) {
	dict, err := s.store.AddWords(ctx, req.Name, req.Words)
	if err != nil {
		zap.L().Error("add sensitive words failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	return s.dictionaryResponse(ctx, dict)
}

func (s *SensitiveWordService) RemoveSensitiveWords(ctx context.Context, req *proto.RemoveSensitiveWordsRequest) (*proto.SensitiveDictionaryResponse, error) {
	dict, err := s.store.RemoveWords(ctx, req.Name, req.Words)
	if err != nil {
		zap.L().Error("remove sensitive words failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	return s.dictionaryResponse(ctx, dict)
}

func (s *SensitiveWordService) DeleteSensitiveDictionary(ctx context.Context, req *proto.DeleteSensitiveDictionaryRequest) (*proto.DeleteSensitiveDictionaryResponse, error) {
	if err := s.store.DeleteDictionary(ctx, req.Name); err != nil {
		zap.L().Error("delete sensitive dictionary failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}
	snapshot, err := s.reload(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteSensitiveDictionaryResponse{Snapshot: sensitiveSnapshotToProto(snapshot)}, nil
}

func (s *SensitiveWordService) dictionaryResponse(ctx context.Context, dict SensitiveRuntimeDictionary) (*proto.SensitiveDictionaryResponse, error) {
	snapshot, err := s.reload(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.SensitiveDictionaryResponse{Dictionary: sensitiveDictionaryToProto(dict), Snapshot: sensitiveSnapshotToProto(snapshot)}, nil
}

func (s *SensitiveWordService) reload(ctx context.Context) (sensitive.Snapshot, error) {
	snapshot, err := s.policy.Reload(ctx, s.source)
	if err != nil {
		zap.L().Error("reload sensitive words failed", zap.Error(err))
		return s.policy.Snapshot(), nil
	}
	return snapshot, nil
}

func sensitiveDictionarySummaryToProto(dict SensitiveRuntimeDictionary) *proto.SensitiveDictionarySummary {
	return &proto.SensitiveDictionarySummary{
		Name:         dict.Name,
		EffectModels: append([]string(nil), dict.EffectModels...),
		Enabled:      dict.Enabled,
		WordCount:    int32(len(dict.Words)),
	}
}

func sensitiveDictionaryToProto(dict SensitiveRuntimeDictionary) *proto.SensitiveDictionary {
	return &proto.SensitiveDictionary{
		Name:         dict.Name,
		EffectModels: append([]string(nil), dict.EffectModels...),
		Enabled:      dict.Enabled,
		Words:        append([]string(nil), dict.Words...),
	}
}

func sensitiveSnapshotToProto(snapshot sensitive.Snapshot) *proto.SensitiveWordSnapshot {
	return &proto.SensitiveWordSnapshot{
		Enabled:         snapshot.Enabled,
		Version:         snapshot.Version,
		LoadedAt:        timestamppb.New(snapshot.LoadedAt),
		DictionaryCount: int32(snapshot.DictionaryCount),
	}
}
