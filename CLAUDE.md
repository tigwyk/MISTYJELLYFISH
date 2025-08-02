# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Operation MISTY JELLYFISH is an agnostic Bluesky reply bot written in Python. The bot monitors the Bluesky timeline and automatically replies to posts using AI-generated responses via LM Studio integration. Users can configure custom keywords and regex patterns to target specific posts.

## Development Commands

### Setup
```bash
# Create and activate virtual environment
python3 -m venv venv
source venv/bin/activate  # On macOS/Linux
# venv\Scripts\activate   # On Windows

# Install in development mode
pip install -e .

# Install dev dependencies
pip install -e .[dev]
```

### Running the Bot
```bash
# Copy environment template and fill in credentials
cp .env.example .env
# Edit .env with your Bluesky credentials

# Run the bot
python -m misty_jellyfish.main
# Or use the installed script
misty-jellyfish
```

### Development Tools
```bash
# Format code
black misty_jellyfish/

# Lint code  
flake8 misty_jellyfish/

# Type checking
mypy misty_jellyfish/

# Run tests
pytest
```

## Architecture

### Core Components
- `misty_jellyfish/bot.py`: Main bot implementation with Bluesky API integration
- `misty_jellyfish/main.py`: Entry point with configuration and signal handling
- Uses `atproto` library for Bluesky AT Protocol communication

### Bot Behavior
- Authenticates with Bluesky using handle/password
- Monitors timeline at configurable intervals (default: 60 seconds)
- Matches posts using configurable keywords and regex patterns
- Generates AI replies using LM Studio API
- Avoids replying to its own posts
- Graceful shutdown on SIGINT/SIGTERM

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