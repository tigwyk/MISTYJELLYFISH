// Package bot implements the MISTY JELLYFISH Bluesky reply bot.
package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tigwyk/mistyjellyfish/internal/config"
)

const defaultBskyHost = "https://bsky.social"

// session holds the authenticated AT Protocol session tokens.
type session struct {
	AccessJWT  string `json:"accessJwt"`
	RefreshJWT string `json:"refreshJwt"`
	Handle     string `json:"handle"`
	DID        string `json:"did"`
}

// StrongRef is an AT Protocol strong reference (cid + uri).
type StrongRef struct {
	CID string `json:"cid"`
	URI string `json:"uri"`
}

// ReplyRef is the reply field of an AppBskyFeedPost record.
type ReplyRef struct {
	Root   StrongRef `json:"root"`
	Parent StrongRef `json:"parent"`
}

// feedPost is the record type for a Bluesky post.
type feedPost struct {
	Type      string    `json:"$type"`
	Text      string    `json:"text"`
	Reply     *ReplyRef `json:"reply,omitempty"`
	CreatedAt string    `json:"createdAt"`
}

// timelineResponse is the API response from app.bsky.feed.getTimeline.
type timelineResponse struct {
	Feed []FeedViewPost `json:"feed"`
}

// FeedViewPost wraps a post in the timeline feed.
type FeedViewPost struct {
	Post PostView `json:"post"`
}

// PostView holds post metadata and the record.
type PostView struct {
	URI    string     `json:"uri"`
	CID    string     `json:"cid"`
	Author AuthorView `json:"author"`
	Record FeedRecord `json:"record"`
}

// AuthorView holds author information.
type AuthorView struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

// FeedRecord is the raw record JSON of a post, including optional reply field.
type FeedRecord struct {
	Type  string    `json:"$type"`
	Text  string    `json:"text"`
	Reply *ReplyRef `json:"reply,omitempty"`
}

// chatMessage is a message for the LLM API.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the request body for the OpenAI-compatible chat API.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// chatResponse is the response body from the OpenAI-compatible chat API.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Bot is the MISTY JELLYFISH Bluesky reply bot.
type Bot struct {
	handle   string
	password string
	config   config.BotConfig
	patterns []*regexp.Regexp
	sess     *session
	client   *http.Client
	bskyHost string
}

// New creates a Bot from the supplied credentials and configuration,
// connecting to the default Bluesky host (https://bsky.social).
func New(handle, password string, cfg config.BotConfig) (*Bot, error) {
	return NewWithHost(defaultBskyHost, handle, password, cfg)
}

// NewWithHost creates a Bot like New but with a configurable Bluesky host URL.
// This is useful for testing against a local httptest.Server.
func NewWithHost(bskyHost, handle, password string, cfg config.BotConfig) (*Bot, error) {
	patterns, err := compilePatterns(cfg.RegexPatterns)
	if err != nil {
		return nil, err
	}

	return &Bot{
		handle:   handle,
		password: password,
		config:   cfg,
		patterns: patterns,
		client:   &http.Client{Timeout: 30 * time.Second},
		bskyHost: bskyHost,
	}, nil
}

// MakeFeedViewPost constructs a FeedViewPost for use in tests.
func MakeFeedViewPost(handle, text, uri, cid string, reply *ReplyRef) FeedViewPost {
	return FeedViewPost{
		Post: PostView{
			URI: uri,
			CID: cid,
			Author: AuthorView{
				Handle: handle,
			},
			Record: FeedRecord{
				Type:  "app.bsky.feed.post",
				Text:  text,
				Reply: reply,
			},
		},
	}
}

// compilePatterns compiles regex patterns, logging and skipping invalid ones.
func compilePatterns(raw []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(raw))
	for _, p := range raw {
		compiled, err := regexp.Compile("(?i)" + p)
		if err != nil {
			log.Printf("Warning: invalid regex pattern %q: %v", p, err)
			continue
		}
		out = append(out, compiled)
	}
	return out, nil
}

// Run starts the bot loop. It blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	log.Println("Starting MISTY JELLYFISH reply bot...")

	if err := b.Authenticate(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	interval := time.Duration(b.config.ReplySettings.CheckInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately, then on each tick.
	b.MonitorPosts(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping reply bot...")
			return nil
		case <-ticker.C:
			b.MonitorPosts(ctx)
		}
	}
}

// Authenticate creates an AT Protocol session.
func (b *Bot) Authenticate(ctx context.Context) error {
	log.Printf("Authenticating as %s...", b.handle)

	body, _ := json.Marshal(map[string]string{
		"identifier": b.handle,
		"password":   b.password,
	})

	resp, err := b.xrpcPost(ctx, "/xrpc/com.atproto.server.createSession", "", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("createSession returned %d: %s", resp.StatusCode, raw)
	}

	var s session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return fmt.Errorf("decoding session response: %w", err)
	}

	b.sess = &s
	log.Printf("Authenticated as %s (DID: %s)", s.Handle, s.DID)
	return nil
}

// MonitorPosts fetches the timeline and processes matching posts.
func (b *Bot) MonitorPosts(ctx context.Context) {
	if b.sess == nil {
		log.Println("Not authenticated, skipping post monitoring")
		return
	}

	if !b.config.ReplySettings.EnableReplies {
		log.Println("Replies disabled in configuration")
		return
	}

	timeline, err := b.getTimeline(ctx)
	if err != nil {
		log.Printf("Error fetching timeline: %v", err)
		return
	}

	for _, item := range timeline.Feed {
		if !b.ShouldReplyTo(item) {
			continue
		}

		replyText, err := b.GenerateReply(ctx, item)
		if err != nil {
			log.Printf("Error generating reply: %v", err)
			continue
		}
		if replyText == "" {
			continue
		}

		if err := b.sendReply(ctx, item, replyText); err != nil {
			log.Printf("Error sending reply: %v", err)
		}
	}
}

// getTimeline fetches the authenticated user's home timeline.
func (b *Bot) getTimeline(ctx context.Context) (*timelineResponse, error) {
	url := fmt.Sprintf("%s/xrpc/app.bsky.feed.getTimeline?limit=%d",
		b.bskyHost, b.config.ReplySettings.TimelineLimit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.sess.AccessJWT)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getTimeline returned %d: %s", resp.StatusCode, raw)
	}

	var tl timelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&tl); err != nil {
		return nil, fmt.Errorf("decoding timeline response: %w", err)
	}
	return &tl, nil
}

// ShouldReplyTo returns true when the bot should reply to the given post.
func (b *Bot) ShouldReplyTo(item FeedViewPost) bool {
	// Skip our own posts.
	if item.Post.Author.Handle == b.handle {
		return false
	}

	// Skip posts that are already replies (to keep things tidy).
	if item.Post.Record.Reply != nil {
		return false
	}

	text := strings.ToLower(item.Post.Record.Text)

	for _, kw := range b.config.Keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			log.Printf("Keyword match %q in post by @%s", kw, item.Post.Author.Handle)
			return true
		}
	}

	for _, p := range b.patterns {
		if p.MatchString(text) {
			log.Printf("Regex match %q in post by @%s", p.String(), item.Post.Author.Handle)
			return true
		}
	}

	return false
}

// GenerateReply calls the LM Studio (OpenAI-compatible) API and returns the reply text.
func (b *Bot) GenerateReply(ctx context.Context, item FeedViewPost) (string, error) {
	author := item.Post.Author.Handle
	postText := item.Post.Record.Text

	payload := chatRequest{
		Model: b.config.LLMAPI.Model,
		Messages: []chatMessage{
			{Role: "system", Content: b.config.LLMAPI.SystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Reply to this post by @%s: %q", author, postText)},
		},
		MaxTokens:   b.config.LLMAPI.MaxTokens,
		Temperature: b.config.LLMAPI.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := b.config.LLMAPI.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling LLM API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decoding LLM response: %w", err)
	}

	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("LLM API returned no choices")
	}

	replyText := strings.TrimSpace(cr.Choices[0].Message.Content)
	log.Printf("Generated reply for @%s: %.50s...", author, replyText)
	return replyText, nil
}

// sendReply posts a reply to the given post.
func (b *Bot) sendReply(ctx context.Context, item FeedViewPost, replyText string) error {
	parent := StrongRef{CID: item.Post.CID, URI: item.Post.URI}

	// Determine root: if the original post has a reply chain, use its root.
	root := parent
	if item.Post.Record.Reply != nil {
		root = item.Post.Record.Reply.Root
	}

	record := feedPost{
		Type:      "app.bsky.feed.post",
		Text:      replyText,
		Reply:     &ReplyRef{Root: root, Parent: parent},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	payload := map[string]interface{}{
		"repo":       b.sess.DID,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := b.xrpcPost(ctx, "/xrpc/com.atproto.repo.createRecord", b.sess.AccessJWT, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("createRecord returned %d: %s", resp.StatusCode, raw)
	}

	log.Printf("Sent reply to @%s: %s", item.Post.Author.Handle, replyText)
	return nil
}

// xrpcPost is a helper that sends a JSON POST to the Bluesky XRPC endpoint.
func (b *Bot) xrpcPost(ctx context.Context, path, accessJWT string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.bskyHost+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessJWT != "" {
		req.Header.Set("Authorization", "Bearer "+accessJWT)
	}
	return b.client.Do(req)
}
