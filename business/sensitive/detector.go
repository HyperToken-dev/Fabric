package sensitive

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudflare/ahocorasick"
)

type Dictionary struct {
	Name         string
	Words        []string
	EffectModels []string
}

type Match struct {
	Dictionary string
	Words      []string
}

type Result struct {
	Matches []Match
}

func (r Result) Rejected() bool {
	return len(r.Matches) > 0
}

type Detector struct {
	dictionaries []compiledDictionary
}

type compiledDictionary struct {
	name         string
	words        []string
	effectModels map[string]struct{}
	matcher      *ahocorasick.Matcher
}

func LoadDictionary(name, path string, effectModels []string) (Dictionary, error) {
	file, err := os.Open(path)
	if err != nil {
		return Dictionary{}, fmt.Errorf("open sensitive dictionary %q: %w", path, err)
	}

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			words = append(words, word)
		}
	}
	scanErr := scanner.Err()
	closeErr := file.Close()
	if scanErr != nil || closeErr != nil {
		return Dictionary{}, errors.Join(scanErr, closeErr)
	}

	words = removeDuplicateStrings(words)
	if len(words) == 0 {
		return Dictionary{}, fmt.Errorf("sensitive dictionary %q contains no usable words", path)
	}
	return Dictionary{
		Name:         name,
		Words:        words,
		EffectModels: append([]string(nil), effectModels...),
	}, nil
}

func NewDetector(dicts ...Dictionary) (*Detector, error) {
	compiled := make([]compiledDictionary, 0, len(dicts))
	for _, dict := range dicts {
		if strings.TrimSpace(dict.Name) == "" {
			return nil, errors.New("sensitive dictionary name is required")
		}

		words := normalizeNonEmptyStrings(dict.Words)
		if len(words) == 0 {
			return nil, fmt.Errorf("sensitive dictionary %q contains no usable words", dict.Name)
		}

		models := make(map[string]struct{})
		for _, model := range removeDuplicateStrings(normalizeNonEmptyStrings(dict.EffectModels)) {
			models[model] = struct{}{}
		}
		compiled = append(compiled, compiledDictionary{
			name:         dict.Name,
			words:        words,
			effectModels: models,
			matcher:      ahocorasick.NewStringMatcher(words),
		})
	}
	return &Detector{dictionaries: compiled}, nil
}

func (d *Detector) Detect(model, text string) Result {
	if d == nil {
		return Result{}
	}

	var result Result
	for _, dict := range d.dictionaries {
		if len(dict.effectModels) > 0 {
			if _, applies := dict.effectModels[model]; !applies {
				continue
			}
		}

		indices := dict.matcher.Match([]byte(text))
		if len(indices) == 0 {
			continue
		}
		words := make([]string, 0, len(indices))
		seen := make(map[int]struct{}, len(indices))
		for _, index := range indices {
			if _, exists := seen[index]; exists {
				continue
			}
			seen[index] = struct{}{}
			words = append(words, dict.words[index])
		}
		result.Matches = append(result.Matches, Match{Dictionary: dict.name, Words: words})
	}
	return result
}

func normalizeNonEmptyStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return removeDuplicateStrings(normalized)
}

func removeDuplicateStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
