"""Main entry point for MISTY JELLYFISH bot"""

import asyncio
import logging
import os
import signal
from dotenv import load_dotenv
from misty_jellyfish.bot import MistyJellyfishBot


def setup_logging():
    """Configure logging"""
    log_level = os.getenv('LOG_LEVEL', 'INFO').upper()
    logging.basicConfig(
        level=getattr(logging, log_level),
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )


async def main():
    """Main function"""
    # Load environment variables from .env file
    load_dotenv()
    
    setup_logging()
    logger = logging.getLogger(__name__)
    
    # Get credentials from environment
    handle = os.getenv('BLUESKY_HANDLE')
    password = os.getenv('BLUESKY_PASSWORD')
    config_path = os.getenv('BOT_CONFIG_PATH', 'bot_config.json')
    
    if not handle or not password:
        logger.error("BLUESKY_HANDLE and BLUESKY_PASSWORD environment variables required")
        logger.error("Copy .env.example to .env and fill in your credentials")
        return 1
    
    bot = MistyJellyfishBot(handle, password, config_path)
    
    # Setup signal handlers for graceful shutdown
    def signal_handler(signum, frame):
        logger.info(f"Received signal {signum}, shutting down gracefully...")
        asyncio.create_task(bot.stop())
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    try:
        await bot.start()
    except KeyboardInterrupt:
        logger.info("Received keyboard interrupt")
    except Exception as e:
        logger.error(f"Bot error: {e}")
        return 1
    
    return 0


if __name__ == "__main__":
    exit_code = asyncio.run(main())
    exit(exit_code)