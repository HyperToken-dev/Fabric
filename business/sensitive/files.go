package sensitive

import (
	"fmt"
	"path/filepath"
	"strings"
)

type DictionaryFileConfig struct {
	Name            string
	EffectModels    []string
	KeywordFileList []string
}

func LoadDetectorFromFiles(dir string, configs []DictionaryFileConfig) (*Detector, error) {
	dictionaries := make([]Dictionary, 0, len(configs))
	for _, dictConfig := range configs {
		name := strings.TrimSpace(dictConfig.Name)
		if name == "" {
			return nil, fmt.Errorf("sensitive dictionary name is required")
		}
		if len(dictConfig.KeywordFileList) == 0 {
			return nil, fmt.Errorf("sensitive dictionary %q keywordFileList is required", name)
		}

		dict := Dictionary{
			Name:         name,
			EffectModels: append([]string(nil), dictConfig.EffectModels...),
		}
		for _, fileName := range dictConfig.KeywordFileList {
			fileName = strings.TrimSpace(fileName)
			if fileName == "" || filepath.Base(fileName) != fileName || fileName == "." || fileName == ".." {
				return nil, fmt.Errorf("invalid sensitive keyword file name %q", fileName)
			}
			loaded, err := LoadDictionary(name, filepath.Join(dir, fileName+".txt"), dictConfig.EffectModels)
			if err != nil {
				return nil, err
			}
			dict.Words = append(dict.Words, loaded.Words...)
		}
		dictionaries = append(dictionaries, dict)
	}
	return NewDetector(dictionaries...)
}
