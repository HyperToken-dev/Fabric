package sensitive

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudflare/ahocorasick"
	"go.uber.org/zap"
)

type Detector struct {
	words   []string
	matcher *ahocorasick.Matcher
}

func NewDetector(words []string) *Detector {
	words = removeDuplicateStrings(words)
	return &Detector{
		words:   words,
		matcher: ahocorasick.NewStringMatcher(words),
	}
}

func LoadWordsFromDir(path string) ([]string, error) {
	words := make([]string, 0)

	if dirExternal, errExternal := os.ReadDir(path); errExternal == nil {
		for _, file := range dirExternal {
			isMatch, _ := regexp.MatchString(`(?i)\.txt$`, file.Name())
			if !isMatch {
				continue
			}
			filePath := filepath.Join(path, file.Name())
			fileObj, err := os.Open(filePath)
			if err != nil {
				zap.S().Warnf("Failed to read sensitive dictionary %s: %v", path, err)
				continue
			}

			scanner := bufio.NewScanner(fileObj)
			for scanner.Scan() {
				word := strings.TrimSpace(scanner.Text())
				if word == "" {
					continue
				}
				words = append(words, word)
			}

			err = fileObj.Close()
			if err != nil {
				return nil, fmt.Errorf("Failed to close external sensitive dictionary: %v", err)
			}

			if err := scanner.Err(); err != nil {
				zap.S().Warnf("Failed to read sensitive dictionary %s: %v", path, err)
			}
		}
	} else {
		zap.S().Warnf("Failed to read external sensitive dictionary: %v.", errExternal)
	}

	return removeDuplicateStrings(words), nil
}

func (d *Detector) Detect(sentence string) bool {
	if d == nil || d.matcher == nil {
		return false
	}
	hitIndices := d.matcher.Match([]byte(sentence))
	if len(hitIndices) > 0 {
		var strBuilder strings.Builder
		fmt.Fprintf(&strBuilder, "Detected %d sensitive keyword(s) for sentence:\n %s.\n", len(hitIndices), sentence)
		for _, index := range hitIndices {
			matchedWord := d.words[index]
			fmt.Fprintf(&strBuilder, "- Blocked by: [%s]\n", matchedWord)
		}
		zap.S().Info(strBuilder.String())
		return true
	}
	return false
}

func removeDuplicateStrings(inputSlice []string) (resultSlice []string) {
	seenMap := make(map[string]bool)
	for _, strVal := range inputSlice {
		if !seenMap[strVal] {
			seenMap[strVal] = true
			resultSlice = append(resultSlice, strVal)
		}
	}
	return resultSlice
}
