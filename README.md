# Operation MISTY JELLYFISH

An agnostic Bluesky reply bot written in Python that automatically responds to posts using AI-generated replies. Fully configurable with custom keywords, regex patterns, and LM Studio integration.

## Features

- 🤖 AI-powered replies using LM Studio API
- 🔍 Configurable keyword and regex pattern matching
- 🔐 Secure authentication with Bluesky AT Protocol
- ⚡ JSON-based configuration system
- 🛡️ Built-in safety filters to avoid spam
- 📊 Comprehensive logging
- 🎯 Flexible targeting system

## Quick Start

1. **Clone and setup**
   ```bash
   git clone <repository-url>
   cd MISTYJELLYFISH
   python3 -m venv venv
   source venv/bin/activate
   pip install -e .
   ```

2. **Setup LM Studio**
   - Download and install [LM Studio](https://lmstudio.ai/)
   - Load a model of your choice
   - Start the local server (default: http://localhost:1234)

3. **Configure credentials**
   ```bash
   cp .env.example .env
   # Edit .env with your Bluesky handle and app password
   ```

4. **Configure bot behavior**
   Edit `bot_config.json` to customize:
   - Keywords to trigger replies
   - Regex patterns for advanced matching
   - LM Studio API settings
   - Reply behavior settings

5. **Run the bot**
   ```bash
   python -m misty_jellyfish.main
   ```

## Configuration

### Environment Variables (.env)
```env
BLUESKY_HANDLE=your.handle.bsky.social
BLUESKY_PASSWORD=your-app-password
LOG_LEVEL=INFO
BOT_CONFIG_PATH=bot_config.json
```

### Bot Configuration (bot_config.json)
```json
{
  "keywords": ["python", "programming", "AI"],
  "regex_patterns": ["\\bcode\\s+review\\b", "\\bhelp\\s+with\\b.*\\b(bug|error)\\b"],
  "llm_api": {
    "base_url": "http://localhost:1234",
    "model": "local-model",
    "system_prompt": "You are a helpful assistant...",
    "max_tokens": 150,
    "temperature": 0.7
  },
  "reply_settings": {
    "check_interval": 60,
    "timeline_limit": 20,
    "enable_replies": true
  }
}
```

## How it Works

1. **Authentication**: Connects to Bluesky using your credentials
2. **Monitoring**: Scans timeline for posts matching your keywords/patterns
3. **AI Generation**: Sends matched posts to LM Studio for reply generation
4. **Reply**: Posts AI-generated responses back to Bluesky
5. **Safety**: Avoids replying to own posts and includes rate limiting

## Development

```bash
# Install dev dependencies
pip install -e .[dev]

# Format code
black misty_jellyfish/

# Lint
flake8 misty_jellyfish/

# Type check
mypy misty_jellyfish/

# Run tests
pytest
```

## LM Studio Setup

1. Download LM Studio from https://lmstudio.ai/
2. Load any compatible model (e.g., Llama, Mistral, etc.)
3. Go to the "Local Server" tab
4. Click "Start Server" (default port 1234)
5. The bot will use the OpenAI-compatible API endpoint

## Safety Features

- Configurable check intervals to avoid rate limits
- Self-reply prevention
- Regex validation with error handling
- Graceful error handling and logging
- Optional reply disable switch
