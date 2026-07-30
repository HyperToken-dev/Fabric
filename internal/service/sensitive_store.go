package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	sensitiveStateFileName           = "state.json"
	sensitiveWordsDirName            = "dictionaries"
	maxSensitiveDictionaryNameLength = 128
	maxSensitiveWordLength           = 256
)

type SensitiveRuntimeState struct {
	Enabled      bool                             `json:"enabled"`
	Dictionaries []SensitiveRuntimeDictionaryMeta `json:"dictionaries"`
}

type SensitiveRuntimeDictionaryMeta struct {
	Name         string   `json:"name"`
	EffectModels []string `json:"effectModels"`
	Enabled      bool     `json:"enabled"`
	KeywordFile  string   `json:"keywordFile"`
}

type SensitiveRuntimeDictionary struct {
	Name         string
	EffectModels []string
	Enabled      bool
	Words        []string
}

type SensitiveRuntimeStore struct {
	mu        sync.Mutex
	baseDir   string
	statePath string
	wordsDir  string
}

func NewSensitiveRuntimeStore(baseDir string) *SensitiveRuntimeStore {
	return &SensitiveRuntimeStore{
		baseDir:   baseDir,
		statePath: filepath.Join(baseDir, sensitiveStateFileName),
		wordsDir:  filepath.Join(baseDir, sensitiveWordsDirName),
	}
}

func (s *SensitiveRuntimeStore) StatePath() string {
	if s == nil {
		return ""
	}
	return s.statePath
}

func (s *SensitiveRuntimeStore) BaseDir() string {
	if s == nil {
		return ""
	}
	return s.baseDir
}

func (s *SensitiveRuntimeStore) WatchPaths() []string {
	if s == nil {
		return nil
	}
	return []string{s.baseDir, s.wordsDir}
}

func (s *SensitiveRuntimeStore) Ensure(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLocked()
}

func (s *SensitiveRuntimeStore) ensureLocked() error {
	if err := os.MkdirAll(s.wordsDir, 0o755); err != nil {
		return fmt.Errorf("create sensitive words directory: %w", err)
	}
	if _, err := os.Stat(s.statePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat sensitive state: %w", err)
	}
	return s.writeStateLocked(SensitiveRuntimeState{Enabled: false, Dictionaries: []SensitiveRuntimeDictionaryMeta{}})
}

func (s *SensitiveRuntimeStore) ReadState(ctx context.Context) (SensitiveRuntimeState, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeState{}, err
	}
	return s.readStateLocked()
}

func (s *SensitiveRuntimeStore) ListDictionaries(ctx context.Context) ([]SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return nil, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return nil, err
	}
	dictionaries := make([]SensitiveRuntimeDictionary, 0, len(state.Dictionaries))
	for _, meta := range state.Dictionaries {
		words, err := s.readWordsLocked(meta.KeywordFile)
		if err != nil {
			return nil, err
		}
		dictionaries = append(dictionaries, SensitiveRuntimeDictionary{
			Name:         meta.Name,
			EffectModels: append([]string(nil), meta.EffectModels...),
			Enabled:      meta.Enabled,
			Words:        words,
		})
	}
	return dictionaries, nil
}

func (s *SensitiveRuntimeStore) GetDictionary(ctx context.Context, name string) (SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	meta, _, ok := findSensitiveDictionary(state.Dictionaries, name)
	if !ok {
		return SensitiveRuntimeDictionary{}, fmt.Errorf("sensitive dictionary %q not found", name)
	}
	words, err := s.readWordsLocked(meta.KeywordFile)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	return SensitiveRuntimeDictionary{Name: meta.Name, EffectModels: append([]string(nil), meta.EffectModels...), Enabled: meta.Enabled, Words: words}, nil
}

func (s *SensitiveRuntimeStore) SetEnabled(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return err
	}
	state.Enabled = enabled
	return s.writeStateLocked(state)
}

func (s *SensitiveRuntimeStore) CreateDictionary(ctx context.Context, name string, effectModels []string, enabled bool, words []string) (SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	effectModels = normalizeRuntimeValues(effectModels)
	words, err := normalizeSensitiveWords(words)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	if _, _, ok := findSensitiveDictionary(state.Dictionaries, name); ok {
		return SensitiveRuntimeDictionary{}, fmt.Errorf("sensitive dictionary %q already exists", name)
	}
	keywordFile, err := s.newKeywordFileNameLocked(state)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	meta := SensitiveRuntimeDictionaryMeta{Name: name, EffectModels: effectModels, Enabled: enabled, KeywordFile: keywordFile}
	if err := s.writeWordsLocked(meta.KeywordFile, words); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state.Dictionaries = append(state.Dictionaries, meta)
	sortSensitiveDictionaries(state.Dictionaries)
	if err := s.writeStateLocked(state); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	return SensitiveRuntimeDictionary{Name: name, EffectModels: effectModels, Enabled: enabled, Words: words}, nil
}

func (s *SensitiveRuntimeStore) UpdateEffectModels(ctx context.Context, name string, effectModels []string) (SensitiveRuntimeDictionary, error) {
	return s.updateDictionaryMeta(ctx, name, func(meta *SensitiveRuntimeDictionaryMeta) {
		meta.EffectModels = normalizeRuntimeValues(effectModels)
	})
}

func (s *SensitiveRuntimeStore) UpdateDictionaryEnabled(ctx context.Context, name string, enabled bool) (SensitiveRuntimeDictionary, error) {
	return s.updateDictionaryMeta(ctx, name, func(meta *SensitiveRuntimeDictionaryMeta) {
		meta.Enabled = enabled
	})
}

func (s *SensitiveRuntimeStore) AddWords(ctx context.Context, name string, words []string) (SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	words, err := normalizeSensitiveWords(words)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	meta, _, ok := findSensitiveDictionary(state.Dictionaries, name)
	if !ok {
		return SensitiveRuntimeDictionary{}, fmt.Errorf("sensitive dictionary %q not found", name)
	}
	current, err := s.readWordsLocked(meta.KeywordFile)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	merged := append(current, words...)
	merged, err = normalizeSensitiveWords(merged)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	if err := s.writeWordsLocked(meta.KeywordFile, merged); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	return SensitiveRuntimeDictionary{Name: meta.Name, EffectModels: append([]string(nil), meta.EffectModels...), Enabled: meta.Enabled, Words: merged}, nil
}

func (s *SensitiveRuntimeStore) RemoveWords(ctx context.Context, name string, words []string) (SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	words = normalizeRuntimeValues(words)
	if len(words) == 0 {
		return SensitiveRuntimeDictionary{}, errors.New("sensitive words are required")
	}
	remove := make(map[string]struct{}, len(words))
	for _, word := range words {
		remove[word] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	meta, _, ok := findSensitiveDictionary(state.Dictionaries, name)
	if !ok {
		return SensitiveRuntimeDictionary{}, fmt.Errorf("sensitive dictionary %q not found", name)
	}
	current, err := s.readWordsLocked(meta.KeywordFile)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	next := make([]string, 0, len(current))
	for _, word := range current {
		if _, shouldRemove := remove[word]; shouldRemove {
			continue
		}
		next = append(next, word)
	}
	if err := s.writeWordsLocked(meta.KeywordFile, next); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	return SensitiveRuntimeDictionary{Name: meta.Name, EffectModels: append([]string(nil), meta.EffectModels...), Enabled: meta.Enabled, Words: next}, nil
}

func (s *SensitiveRuntimeStore) DeleteDictionary(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return err
	}
	meta, index, ok := findSensitiveDictionary(state.Dictionaries, name)
	if !ok {
		return fmt.Errorf("sensitive dictionary %q not found", name)
	}
	wordsPath := s.wordsPath(meta.KeywordFile)
	wordsData, readErr := os.ReadFile(wordsPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read sensitive words file before delete: %w", readErr)
	}
	if err := os.Remove(wordsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove sensitive words file: %w", err)
	}
	state.Dictionaries = append(state.Dictionaries[:index], state.Dictionaries[index+1:]...)
	if err := s.writeStateLocked(state); err != nil {
		if readErr == nil {
			if restoreErr := writeFileAtomic(wordsPath, wordsData, 0o644); restoreErr != nil {
				return fmt.Errorf("write sensitive state: %w; restore sensitive words file: %v", err, restoreErr)
			}
		}
		return err
	}
	return nil
}

func (s *SensitiveRuntimeStore) updateDictionaryMeta(ctx context.Context, name string, update func(*SensitiveRuntimeDictionaryMeta)) (SensitiveRuntimeDictionary, error) {
	if err := ctx.Err(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	name = strings.TrimSpace(name)
	if err := validateSensitiveDictionaryName(name); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	state, err := s.readStateLocked()
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	meta, index, ok := findSensitiveDictionary(state.Dictionaries, name)
	if !ok {
		return SensitiveRuntimeDictionary{}, fmt.Errorf("sensitive dictionary %q not found", name)
	}
	update(&meta)
	state.Dictionaries[index] = meta
	if err := s.writeStateLocked(state); err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	words, err := s.readWordsLocked(meta.KeywordFile)
	if err != nil {
		return SensitiveRuntimeDictionary{}, err
	}
	return SensitiveRuntimeDictionary{Name: meta.Name, EffectModels: append([]string(nil), meta.EffectModels...), Enabled: meta.Enabled, Words: words}, nil
}

func (s *SensitiveRuntimeStore) readStateLocked() (SensitiveRuntimeState, error) {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		return SensitiveRuntimeState{}, fmt.Errorf("read sensitive state: %w", err)
	}
	var state SensitiveRuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return SensitiveRuntimeState{}, fmt.Errorf("parse sensitive state: %w", err)
	}
	if state.Dictionaries == nil {
		state.Dictionaries = []SensitiveRuntimeDictionaryMeta{}
	}
	for i := range state.Dictionaries {
		if err := validateSensitiveDictionaryName(state.Dictionaries[i].Name); err != nil {
			return SensitiveRuntimeState{}, err
		}
		if state.Dictionaries[i].KeywordFile == "" {
			state.Dictionaries[i].KeywordFile = state.Dictionaries[i].Name + ".txt"
		}
		if err := validateSensitiveKeywordFile(state.Dictionaries[i].KeywordFile); err != nil {
			return SensitiveRuntimeState{}, err
		}
		state.Dictionaries[i].EffectModels = normalizeRuntimeValues(state.Dictionaries[i].EffectModels)
	}
	return state, nil
}

func (s *SensitiveRuntimeStore) writeStateLocked(state SensitiveRuntimeState) error {
	if state.Dictionaries == nil {
		state.Dictionaries = []SensitiveRuntimeDictionaryMeta{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sensitive state: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(s.statePath, data, 0o644)
}

func (s *SensitiveRuntimeStore) readWordsLocked(fileName string) ([]string, error) {
	if err := validateSensitiveKeywordFile(fileName); err != nil {
		return nil, err
	}
	file, err := os.Open(s.wordsPath(fileName))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("open sensitive words file: %w", err)
	}
	defer file.Close()
	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			words = append(words, word)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan sensitive words file: %w", err)
	}
	return normalizeSensitiveWords(words)
}

func (s *SensitiveRuntimeStore) writeWordsLocked(fileName string, words []string) error {
	if err := validateSensitiveKeywordFile(fileName); err != nil {
		return err
	}
	words, err := normalizeSensitiveWords(words)
	if err != nil {
		return err
	}
	content := strings.Join(words, "\n")
	if content != "" {
		content += "\n"
	}
	return writeFileAtomic(s.wordsPath(fileName), []byte(content), 0o644)
}

func (s *SensitiveRuntimeStore) wordsPath(fileName string) string {
	return filepath.Join(s.wordsDir, fileName)
}

func (s *SensitiveRuntimeStore) newKeywordFileNameLocked(state SensitiveRuntimeState) (string, error) {
	used := make(map[string]struct{}, len(state.Dictionaries))
	for _, dict := range state.Dictionaries {
		used[dict.KeywordFile] = struct{}{}
	}
	for range 8 {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("generate sensitive keyword file name: %w", err)
		}
		fileName := "dict-" + hex.EncodeToString(bytes) + ".txt"
		if _, exists := used[fileName]; exists {
			continue
		}
		if _, err := os.Stat(s.wordsPath(fileName)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat sensitive keyword file: %w", err)
		}
		return fileName, nil
	}
	return "", errors.New("generate sensitive keyword file name: exhausted attempts")
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}

func findSensitiveDictionary(dicts []SensitiveRuntimeDictionaryMeta, name string) (SensitiveRuntimeDictionaryMeta, int, bool) {
	for i, dict := range dicts {
		if dict.Name == name {
			return dict, i, true
		}
	}
	return SensitiveRuntimeDictionaryMeta{}, -1, false
}

func sortSensitiveDictionaries(dicts []SensitiveRuntimeDictionaryMeta) {
	sort.Slice(dicts, func(i, j int) bool {
		return dicts[i].Name < dicts[j].Name
	})
}

func validateSensitiveDictionaryName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("sensitive dictionary name is required")
	}
	if len([]rune(name)) > maxSensitiveDictionaryNameLength {
		return fmt.Errorf("sensitive dictionary name must be at most %d characters", maxSensitiveDictionaryNameLength)
	}
	return nil
}

func validateSensitiveKeywordFile(fileName string) error {
	if strings.TrimSpace(fileName) == "" || filepath.Base(fileName) != fileName || fileName == "." || fileName == ".." || !strings.HasSuffix(fileName, ".txt") {
		return fmt.Errorf("invalid sensitive keyword file name %q", fileName)
	}
	return nil
}

func normalizeRuntimeValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeSensitiveWords(words []string) ([]string, error) {
	result := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if len([]rune(word)) > maxSensitiveWordLength {
			return nil, fmt.Errorf("sensitive word %q exceeds %d characters", word, maxSensitiveWordLength)
		}
		if _, exists := seen[word]; exists {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
	}
	return result, nil
}
