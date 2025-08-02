"""Main bot implementation"""

import asyncio
import logging
import re
import json
import os
from typing import Optional, List, Dict, Any
from atproto import Client, models
import httpx

logger = logging.getLogger(__name__)


class MistyJellyfishBot:
    """Agnostic Bluesky reply bot implementation"""
    
    def __init__(self, handle: str, password: str, config_path: Optional[str] = None):
        self.handle = handle
        self.password = password
        self.client = Client()
        self.running = False
        self.authenticated = False
        self.config = self._load_config(config_path)
        self.compiled_patterns = self._compile_patterns()
    
    def _load_config(self, config_path: Optional[str] = None) -> Dict[str, Any]:
        """Load bot configuration"""
        default_config = {
            "keywords": [],
            "regex_patterns": [],
            "llm_api": {
                "base_url": "http://localhost:1234",
                "model": "local-model",
                "system_prompt": "You are a helpful assistant replying to social media posts. Keep responses brief, friendly, and relevant to the original post.",
                "max_tokens": 100,
                "temperature": 0.7
            },
            "reply_settings": {
                "check_interval": 60,
                "timeline_limit": 20,
                "enable_replies": True
            }
        }
        
        if config_path and os.path.exists(config_path):
            try:
                with open(config_path, 'r') as f:
                    user_config = json.load(f)
                    default_config.update(user_config)
            except Exception as e:
                logger.warning(f"Failed to load config from {config_path}: {e}")
        
        return default_config
    
    def _compile_patterns(self) -> List[re.Pattern]:
        """Compile regex patterns for efficient matching"""
        patterns = []
        for pattern in self.config.get("regex_patterns", []):
            try:
                patterns.append(re.compile(pattern, re.IGNORECASE))
            except re.error as e:
                logger.warning(f"Invalid regex pattern '{pattern}': {e}")
        return patterns
    
    async def start(self):
        """Start the bot"""
        logger.info("Starting reply bot...")
        
        try:
            await self.authenticate()
            self.running = True
            
            interval = self.config["reply_settings"]["check_interval"]
            while self.running:
                await self.monitor_posts()
                await asyncio.sleep(interval)
                
        except Exception as e:
            logger.error(f"Bot error: {e}")
            await self.stop()
    
    async def stop(self):
        """Stop the bot"""
        logger.info("Stopping reply bot...")
        self.running = False
    
    async def authenticate(self):
        """Authenticate with Bluesky"""
        try:
            logger.info(f"Authenticating as {self.handle}...")
            profile = self.client.login(self.handle, self.password)
            self.authenticated = True
            logger.info(f"Successfully authenticated as {profile.display_name or self.handle}")
        except Exception as e:
            logger.error(f"Authentication failed: {e}")
            raise
    
    async def monitor_posts(self):
        """Monitor posts for replies"""
        if not self.authenticated:
            logger.warning("Not authenticated, skipping post monitoring")
            return
            
        if not self.config["reply_settings"]["enable_replies"]:
            logger.debug("Replies disabled in configuration")
            return
            
        try:
            limit = self.config["reply_settings"]["timeline_limit"]
            timeline = self.client.get_timeline(limit=limit)
            
            for post in timeline.feed:
                if self.should_reply_to_post(post):
                    reply_text = await self.generate_reply(post)
                    if reply_text:
                        await self.send_reply(post, reply_text)
                        
        except Exception as e:
            logger.error(f"Error monitoring posts: {e}")
    
    def should_reply_to_post(self, post) -> bool:
        """Determine if we should reply to a post"""
        # Avoid replying to our own posts
        if post.post.author.handle == self.handle:
            return False
        
        # Check if we've already replied (basic check)
        if hasattr(post.post.record, 'reply') and post.post.record.reply:
            return False
            
        post_text = post.post.record.text.lower()
        
        # Check keyword matches
        for keyword in self.config.get("keywords", []):
            if keyword.lower() in post_text:
                logger.debug(f"Keyword match: '{keyword}' in post by {post.post.author.handle}")
                return True
        
        # Check regex pattern matches
        for pattern in self.compiled_patterns:
            if pattern.search(post_text):
                logger.debug(f"Regex match: '{pattern.pattern}' in post by {post.post.author.handle}")
                return True
        
        return False
    
    async def generate_reply(self, post) -> Optional[str]:
        """Generate AI reply using LM Studio API"""
        try:
            post_text = post.post.record.text
            author_handle = post.post.author.handle
            
            # Prepare the prompt
            system_prompt = self.config["llm_api"]["system_prompt"]
            user_prompt = f"Reply to this post by @{author_handle}: \"{post_text}\""
            
            # Call LM Studio API
            async with httpx.AsyncClient() as client:
                response = await client.post(
                    f"{self.config['llm_api']['base_url']}/v1/chat/completions",
                    json={
                        "model": self.config["llm_api"]["model"],
                        "messages": [
                            {"role": "system", "content": system_prompt},
                            {"role": "user", "content": user_prompt}
                        ],
                        "max_tokens": self.config["llm_api"]["max_tokens"],
                        "temperature": self.config["llm_api"]["temperature"]
                    },
                    timeout=30.0
                )
                
                if response.status_code == 200:
                    data = response.json()
                    reply_text = data["choices"][0]["message"]["content"].strip()
                    logger.info(f"Generated reply for @{author_handle}: {reply_text[:50]}...")
                    return reply_text
                else:
                    logger.error(f"LM Studio API error: {response.status_code} - {response.text}")
                    
        except Exception as e:
            logger.error(f"Failed to generate reply: {e}")
        
        return None
    
    async def send_reply(self, original_post, reply_text: str):
        """Send a reply to a post"""
        try:
            # Create reply
            reply_ref = models.create_strong_ref(original_post.post)
            root_ref = original_post.post.record.reply.root if original_post.post.record.reply else reply_ref
            
            self.client.send_post(
                text=reply_text,
                reply_to=models.AppBskyFeedPost.ReplyRef(parent=reply_ref, root=root_ref)
            )
            
            logger.info(f"Sent reply to {original_post.post.author.handle}: {reply_text}")
            
        except Exception as e:
            logger.error(f"Failed to send reply: {e}")