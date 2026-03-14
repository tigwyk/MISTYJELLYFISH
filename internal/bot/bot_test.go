package bot_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tigwyk/mistyjellyfish/internal/bot"
	"github.com/tigwyk/mistyjellyfish/internal/config"
)

// newTestBot creates a Bot wired to a fake Bluesky server and an optional fake LLM server.
func newTestBot(t *testing.T, bskyServer *httptest.Server, llmServer *httptest.Server, cfg config.BotConfig) *bot.Bot {
	t.Helper()
	if llmServer != nil {
		cfg.LLMAPI.BaseURL = llmServer.URL
	}
	b, err := bot.NewWithHost(bskyServer.URL, "test.bsky.social", "secret", cfg)
	if err != nil {
		t.Fatalf("bot.NewWithHost: %v", err)
	}
	return b
}

// fakeBskyServer returns a test server that handles createSession and getTimeline.
func fakeBskyServer(t *testing.T, feed []map[string]interface{}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/xrpc/com.atproto.server.createSession", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"accessJwt":   "fake-access-token",
			"refreshJwt":  "fake-refresh-token",
			"handle":      "test.bsky.social",
			"did":         "did:plc:testuser",
		})
	})

	mux.HandleFunc("/xrpc/app.bsky.feed.getTimeline", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feed": feed,
		})
	})

	mux.HandleFunc("/xrpc/com.atproto.repo.createRecord", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"uri": "at://did:plc:testuser/app.bsky.feed.post/test123",
			"cid": "bafyreitest",
		})
	})

	return httptest.NewServer(mux)
}

// fakeLLMServer returns a test server that mimics the OpenAI chat completions API.
func fakeLLMServer(t *testing.T, replyContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": replyContent,
					},
				},
			},
		})
	}))
}

func defaultCfg() config.BotConfig {
	cfg := config.DefaultConfig()
	cfg.Keywords = []string{"golang"}
	cfg.ReplySettings.EnableReplies = true
	cfg.ReplySettings.TimelineLimit = 5
	return cfg
}

func TestAuthentication(t *testing.T) {
	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, defaultCfg())
	if err := b.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}
}

func TestShouldReplyTo_KeywordMatch(t *testing.T) {
	cfg := defaultCfg()
	cfg.Keywords = []string{"golang"}

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, cfg)

	post := bot.MakeFeedViewPost("other.user", "I love golang programming", "at://did:plc:other/post/1", "cid1", nil)
	if !b.ShouldReplyTo(post) {
		t.Error("expected ShouldReplyTo=true for keyword match")
	}
}

func TestShouldReplyTo_NoMatch(t *testing.T) {
	cfg := defaultCfg()
	cfg.Keywords = []string{"golang"}

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, cfg)

	post := bot.MakeFeedViewPost("other.user", "just a random post", "at://did:plc:other/post/2", "cid2", nil)
	if b.ShouldReplyTo(post) {
		t.Error("expected ShouldReplyTo=false for no match")
	}
}

func TestShouldReplyTo_OwnPost(t *testing.T) {
	cfg := defaultCfg()
	cfg.Keywords = []string{"golang"}

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, cfg)

	// Same handle as the bot itself.
	post := bot.MakeFeedViewPost("test.bsky.social", "I love golang", "at://did:plc:testuser/post/3", "cid3", nil)
	if b.ShouldReplyTo(post) {
		t.Error("expected ShouldReplyTo=false for bot's own post")
	}
}

func TestShouldReplyTo_AlreadyReply(t *testing.T) {
	cfg := defaultCfg()
	cfg.Keywords = []string{"golang"}

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, cfg)

	replyRef := &bot.ReplyRef{
		Root:   bot.StrongRef{CID: "rootcid", URI: "at://root"},
		Parent: bot.StrongRef{CID: "parentcid", URI: "at://parent"},
	}
	post := bot.MakeFeedViewPost("other.user", "I love golang", "at://did:plc:other/post/4", "cid4", replyRef)
	if b.ShouldReplyTo(post) {
		t.Error("expected ShouldReplyTo=false for post that is already a reply")
	}
}

func TestShouldReplyTo_RegexMatch(t *testing.T) {
	cfg := defaultCfg()
	cfg.Keywords = []string{}
	cfg.RegexPatterns = []string{`\bcode\s+review\b`}

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	b := newTestBot(t, bsky, nil, cfg)

	post := bot.MakeFeedViewPost("other.user", "can anyone do a code review for me?", "at://did:plc:other/post/5", "cid5", nil)
	if !b.ShouldReplyTo(post) {
		t.Error("expected ShouldReplyTo=true for regex match")
	}
}

func TestGenerateReply(t *testing.T) {
	llm := fakeLLMServer(t, "Hello from the bot!")
	defer llm.Close()

	bsky := fakeBskyServer(t, nil)
	defer bsky.Close()

	cfg := defaultCfg()
	b := newTestBot(t, bsky, llm, cfg)

	post := bot.MakeFeedViewPost("other.user", "tell me about golang", "at://did:plc:other/post/6", "cid6", nil)
	reply, err := b.GenerateReply(context.Background(), post)
	if err != nil {
		t.Fatalf("GenerateReply() error: %v", err)
	}
	if !strings.Contains(reply, "Hello from the bot!") {
		t.Errorf("unexpected reply: %s", reply)
	}
}

func TestRun_MonitorAndReply(t *testing.T) {
	llm := fakeLLMServer(t, "Great post about golang!")
	defer llm.Close()

	feed := []map[string]interface{}{
		{
			"post": map[string]interface{}{
				"uri": "at://did:plc:other/app.bsky.feed.post/abc",
				"cid": "testcid",
				"author": map[string]string{
					"did":    "did:plc:other",
					"handle": "other.user",
				},
				"record": map[string]interface{}{
					"$type": "app.bsky.feed.post",
					"text":  "I love golang",
				},
			},
		},
	}

	bsky := fakeBskyServer(t, feed)
	defer bsky.Close()

	cfg := defaultCfg()
	b := newTestBot(t, bsky, llm, cfg)

	// Authenticate first so Run can proceed.
	if err := b.Authenticate(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Run a single monitoring cycle (not the full loop).
	b.MonitorPosts(context.Background())
}
