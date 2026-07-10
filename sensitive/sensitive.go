package sensitive

import (
	"bufio"
	"fmt"
	"hyper-token/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudflare/ahocorasick"
	"go.uber.org/zap"
)

var sensitiveWords []string

var matcher *ahocorasick.Matcher

func LoadSensitiveWord(cfg *config.Config) error {
	sensitiveWords = make([]string, 0)

	path := filepath.Join(cfg.WorkDir, "stwd")
	if isPathExists, err := checkDirExists(path); err == nil {
		if !isPathExists {
			path = filepath.Join(cfg.RunPath, "configs", "stwd")
		}
	}

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
				sensitiveWords = append(sensitiveWords, word)
			}

			err = fileObj.Close()
			if err != nil {
				return fmt.Errorf("Failed to close external sensitive dictionary: %v", err)
			}

			if err := scanner.Err(); err != nil {
				zap.S().Warnf("Failed to read sensitive dictionary %s: %v", path, err)
			}
		}
	} else {
		zap.S().Warnf("Failed to read external sensitive dictionary: %v.", errExternal)
	}

	sensitiveWords = removeDuplicateStrings(sensitiveWords)
	matcher = ahocorasick.NewStringMatcher(sensitiveWords)
	return nil
}

func DetectSensitiveWord(sentence string) bool {
	if matcher == nil {
		return false
	}
	hitIndices := matcher.Match([]byte(sentence))
	if len(hitIndices) > 0 {
		var strBuilder strings.Builder
		fmt.Fprintf(&strBuilder, "Detected %d sensitive keyword(s) for sentence:\n %s.\n", len(hitIndices), sentence)
		for _, index := range hitIndices {
			matchedWord := sensitiveWords[index]
			fmt.Fprintf(&strBuilder, "- Blocked by: [%s]\n", matchedWord)
		}
		zap.S().Warn(strBuilder.String())
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

func checkDirExists(path string) (isExist bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}
