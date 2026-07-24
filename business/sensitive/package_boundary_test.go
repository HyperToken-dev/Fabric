package sensitive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSensitivePackageDoesNotImportGatewayConfig(t *testing.T) {
	forbidden := []string{
		"github.com/HyperToken-dev/fabric/internal/config",
		"github.com/spf13/viper",
		"go.uber.org/zap",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if path == "package_boundary_test.go" {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range forbidden {
			if strings.Contains(string(content), value) {
				t.Fatalf("%s imports or references forbidden dependency %q", path, value)
			}
		}
	}
}
