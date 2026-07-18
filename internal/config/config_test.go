package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.BaseURL != defaultBaseURL || cfg.LLM.Model != defaultModel {
		t.Errorf("默认值错误: %+v", cfg.LLM)
	}
}

func TestLoadEnv(t *testing.T) {
	t.Setenv("STOCKAGENT_API_KEY", "envkey")
	t.Setenv("STOCKAGENT_MODEL", "env-model")
	cfg, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.APIKey != "envkey" || cfg.LLM.Model != "env-model" {
		t.Errorf("环境变量未生效: %+v", cfg.LLM)
	}
}

func TestLoadFileAndFlagOverride(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "c.yaml")
	content := "llm:\n  base_url: https://x.example.com\n  api_key: filekey\n  model: m1\n"
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(f, Overrides{Model: "m2"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.BaseURL != "https://x.example.com" || cfg.LLM.APIKey != "filekey" {
		t.Errorf("配置文件未生效: %+v", cfg.LLM)
	}
	if cfg.LLM.Model != "m2" {
		t.Errorf("flag 应覆盖配置文件: model = %q", cfg.LLM.Model)
	}
}

func TestLoadMissingExplicitFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml"), Overrides{}); err == nil {
		t.Error("显式指定不存在的配置文件应报错")
	}
}
