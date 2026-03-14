# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Operation MISTY JELLYFISH is an agnostic Bluesky reply bot written in Go. The bot monitors the Bluesky timeline and automatically replies to posts using AI-generated responses via LM Studio integration. Users can configure custom keywords and regex patterns to target specific posts.

## Development Commands

### Setup
```bash
# Ensure Go 1.24+ is installed
go version

# Download dependencies
go mod download
```

### Running the Bot
```bash
# Copy environment template and fill in credentials
cp .env.example .env
# Edit .env with your Bluesky credentials

# Build and run
go build -o misty-jellyfish ./cmd/misty-jellyfish
./misty-jellyfish

# Or run directly
go run ./cmd/misty-jellyfish
```

### Development Tools
```bash
# Vet (lint)
go vet ./...

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Build
go build ./...
```

## Architecture

### Core Components
- `cmd/misty-jellyfish/main.go`: Entry point with configuration and signal handling
- `internal/bot/bot.go`: Main bot implementation with Bluesky AT Protocol API calls
- `internal/config/config.go`: Configuration types and loading from bot_config.json
- Bluesky API calls made directly via `net/http` (no external AT Protocol library required)

### Bot Behavior
- Authenticates with Bluesky using handle/password (AT Protocol createSession)
- Monitors timeline at configurable intervals (default: 60 seconds)
- Matches posts using configurable keywords and regex patterns
- Generates AI replies using LM Studio API (OpenAI-compatible)
- Avoids replying to its own posts and to posts that are already replies
- Graceful shutdown on SIGINT/SIGTERM via signal.NotifyContext

## Configuration

### Environment variables (set in `.env`):
- `BLUESKY_HANDLE`: Your Bluesky handle (e.g., username.bsky.social)
- `BLUESKY_PASSWORD`: Your Bluesky app password
- `LOG_LEVEL`: Logging level (INFO, DEBUG, WARNING, ERROR)
- `BOT_CONFIG_PATH`: Path to JSON config file (default: bot_config.json)

### Bot configuration (bot_config.json):
- `keywords`: Array of strings to match in posts
- `regex_patterns`: Array of regex patterns for advanced matching
- `llm_api`: LM Studio API configuration
  - `base_url`: LM Studio server URL (default: http://localhost:1234)
  - `model`: Model name
  - `system_prompt`: AI system prompt
  - `max_tokens`: Max response tokens
  - `temperature`: Response creativity (0.0-1.0)
- `reply_settings`: Bot behavior settings
  - `check_interval`: Seconds between timeline checks
  - `timeline_limit`: Number of posts to check per cycle
  - `enable_replies`: Toggle to enable/disable actual replies