package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tigwyk/mistyjellyfish/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.LLMAPI.BaseURL != "http://localhost:1234" {
		t.Errorf("unexpected default base_url: %s", cfg.LLMAPI.BaseURL)
	}
	if cfg.ReplySettings.CheckInterval != 60 {
		t.Errorf("unexpected default check_interval: %d", cfg.ReplySettings.CheckInterval)
	}
	if cfg.ReplySettings.TimelineLimit != 20 {
		t.Errorf("unexpected default timeline_limit: %d", cfg.ReplySettings.TimelineLimit)
	}
	if !cfg.ReplySettings.EnableReplies {
		t.Error("expected enable_replies to default to true")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg := config.Load("/nonexistent/path/config.json")
	// Should return defaults without panic.
	if cfg.LLMAPI.BaseURL != "http://localhost:1234" {
		t.Errorf("expected defaults when file missing, got base_url=%s", cfg.LLMAPI.BaseURL)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg := config.Load("")
	if cfg.LLMAPI.MaxTokens != 100 {
		t.Errorf("expected defaults for empty path, got max_tokens=%d", cfg.LLMAPI.MaxTokens)
	}
}

func TestLoadValidJSON(t *testing.T) {
	data := config.BotConfig{
		Keywords:      []string{"go", "golang"},
		RegexPatterns: []string{"\\btest\\b"},
		LLMAPI: config.LLMAPIConfig{
			BaseURL:      "http://custom:5678",
			Model:        "my-model",
			SystemPrompt: "Be concise.",
			MaxTokens:    50,
			Temperature:  0.5,
		},
		ReplySettings: config.ReplySettings{
			CheckInterval: 30,
			TimelineLimit: 10,
			EnableReplies: false,
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(raw); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg := config.Load(f.Name())

	if cfg.LLMAPI.BaseURL != "http://custom:5678" {
		t.Errorf("expected base_url=http://custom:5678, got %s", cfg.LLMAPI.BaseURL)
	}
	if len(cfg.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(cfg.Keywords))
	}
	if cfg.ReplySettings.EnableReplies {
		t.Error("expected enable_replies=false")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Load(path)
	// Should fall back to defaults.
	if cfg.LLMAPI.BaseURL != "http://localhost:1234" {
		t.Errorf("expected defaults for invalid JSON, got base_url=%s", cfg.LLMAPI.BaseURL)
	}
}
