// Package config 加载 stockagent 的配置文件与环境变量。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LLMConfig 描述 OpenAI 兼容协议的 LLM 接入配置。
type LLMConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

// Config 是顶层配置。
type Config struct {
	LLM LLMConfig `yaml:"llm"`
}

// Overrides 为命令行 flag 覆盖项，空字符串表示不覆盖。
type Overrides struct {
	BaseURL string
	APIKey  string
	Model   string
}

const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
)

// Load 按 默认值 < 配置文件 < 环境变量 < flag 的优先级合并配置。
func Load(explicitPath string, ov Overrides) (*Config, error) {
	cfg := &Config{}
	cfg.LLM.BaseURL = defaultBaseURL
	cfg.LLM.Model = defaultModel

	path, err := findFile(explicitPath)
	if err != nil {
		return nil, err
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
		}
	}

	if v := os.Getenv("STOCKAGENT_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("STOCKAGENT_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("STOCKAGENT_MODEL"); v != "" {
		cfg.LLM.Model = v
	}

	if ov.BaseURL != "" {
		cfg.LLM.BaseURL = ov.BaseURL
	}
	if ov.APIKey != "" {
		cfg.LLM.APIKey = ov.APIKey
	}
	if ov.Model != "" {
		cfg.LLM.Model = ov.Model
	}
	return cfg, nil
}

// findFile 返回找到的配置文件路径；未找到返回空字符串（视为使用默认值）。
func findFile(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("配置文件 %s 不存在", explicit)
		}
		return explicit, nil
	}
	candidates := []string{"stockagent.yaml", "stockagent.yml"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".stockagent.yaml"),
			filepath.Join(home, ".config", "stockagent", "config.yaml"),
		)
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", nil
}
