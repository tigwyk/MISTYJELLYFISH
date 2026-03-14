// Package config provides configuration loading for the MISTY JELLYFISH bot.
package config

import (
	"encoding/json"
	"log"
	"os"
)

// LLMAPIConfig holds settings for the LM Studio / OpenAI-compatible API.
type LLMAPIConfig struct {
	BaseURL      string  `json:"base_url"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"system_prompt"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
}

// ReplySettings holds bot behaviour settings.
type ReplySettings struct {
	CheckInterval int  `json:"check_interval"`
	TimelineLimit int  `json:"timeline_limit"`
	EnableReplies bool `json:"enable_replies"`
}

// BotConfig is the top-level configuration structure loaded from bot_config.json.
type BotConfig struct {
	Keywords      []string      `json:"keywords"`
	RegexPatterns []string      `json:"regex_patterns"`
	LLMAPI        LLMAPIConfig  `json:"llm_api"`
	ReplySettings ReplySettings `json:"reply_settings"`
}

// DefaultConfig returns a BotConfig populated with sensible defaults.
func DefaultConfig() BotConfig {
	return BotConfig{
		Keywords:      []string{},
		RegexPatterns: []string{},
		LLMAPI: LLMAPIConfig{
			BaseURL:      "http://localhost:1234",
			Model:        "local-model",
			SystemPrompt: "You are a helpful assistant replying to social media posts. Keep responses brief, friendly, and relevant to the original post.",
			MaxTokens:    100,
			Temperature:  0.7,
		},
		ReplySettings: ReplySettings{
			CheckInterval: 60,
			TimelineLimit: 20,
			EnableReplies: true,
		},
	}
}

// Load reads a BotConfig from the JSON file at configPath. If the file does
// not exist or cannot be parsed the default config is returned.
func Load(configPath string) BotConfig {
	cfg := DefaultConfig()

	if configPath == "" {
		return cfg
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Warning: could not read config file %q: %v", configPath, err)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Warning: could not parse config file %q: %v", configPath, err)
		return cfg
	}

	return cfg
}
