package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadSensitiveDictionaries(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("proxyAddr", 3002)
	viper.Set("adminAddr", 9090)
	viper.Set("sensitiveWordDetect", true)
	viper.Set("sensitiveWordDictionaries", []map[string]any{
		{"name": "common", "effectModels": []string{}, "keywordFileList": []string{"common"}},
		{"name": "scoped", "effectModels": []string{"gpt-5.5"}, "keywordFileList": []string{"scoped", "shared"}},
	})

	cfg, err := Load("work", "run")
	if err != nil {
		t.Fatal(err)
	}
	want := []SensitiveDictionaryConfig{
		{Name: "common", EffectModels: []string{}, KeywordFileList: []string{"common"}},
		{Name: "scoped", EffectModels: []string{"gpt-5.5"}, KeywordFileList: []string{"scoped", "shared"}},
	}
	if !reflect.DeepEqual(cfg.SensitiveDictionaries, want) {
		t.Fatalf("SensitiveDictionaries = %#v, want %#v", cfg.SensitiveDictionaries, want)
	}
}

func TestLoadNormalizesAddressesAndRuntimePaths(t *testing.T) {
	tests := []struct {
		name      string
		proxyAddr string
		adminAddr string
		wantProxy string
		wantAdmin string
	}{
		{name: "without colon", proxyAddr: "3002", adminAddr: "9090", wantProxy: ":3002", wantAdmin: ":9090"},
		{name: "with colon", proxyAddr: ":3002", adminAddr: ":9090", wantProxy: ":3002", wantAdmin: ":9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set("proxyAddr", tt.proxyAddr)
			viper.Set("adminAddr", tt.adminAddr)

			cfg, err := Load("work-dir", "run-path")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ProxyAddr != tt.wantProxy {
				t.Fatalf("ProxyAddr = %q, want %q", cfg.ProxyAddr, tt.wantProxy)
			}
			if cfg.AdminAddr != tt.wantAdmin {
				t.Fatalf("AdminAddr = %q, want %q", cfg.AdminAddr, tt.wantAdmin)
			}
			if cfg.WorkDir != "work-dir" {
				t.Fatalf("WorkDir = %q, want work-dir", cfg.WorkDir)
			}
			if cfg.RunPath != "run-path" {
				t.Fatalf("RunPath = %q, want run-path", cfg.RunPath)
			}
		})
	}
}
