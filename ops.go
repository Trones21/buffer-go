package buffer

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// This file is the typed operations layer: ergonomic Go methods for the handful
// of Buffer operations this project cares about — list channels, create a post,
// list what's queued — built on top of the generic Do transport in buffer.go.
// It stays in this package (not something project-specific) because these are
// generic Buffer operations, not policy about *what* or *when* to post; that
// policy lives in the consumer (the MCP host model, a CLI, your own code).
//
// TODO(introspect): the GraphQL documents below are PLACEHOLDERS. They are
// written against Buffer's documented concepts, not confirmed operation names.
// Before trusting live mode, run Client.Introspect with a real BUFFER_TOKEN,
// confirm the actual query/mutation names + input shapes, and replace the four
// const documents in this file. Nothing else has to change: the Go types, the
// method signatures, and the MCP tools built on them are stable across that fix.

// WhenQueue tells CreatePost to append the post to the channel's Buffer queue,
// letting Buffer's own posting schedule pick the slot. Any other value is
// treated as an explicit RFC3339 timestamp to schedule the post for. This is
// the crux of "when to post": for the common case you don't schedule anything —
// Buffer already knows each channel's posting times, so you just join the queue.
const WhenQueue = "queue"

// Channel is a social account connected to the authenticated Buffer org.
type Channel struct {
	ID      string `json:"id"`
	Service string `json:"service"` // e.g. "twitter", "linkedin", "instagram"
	Name    string `json:"name"`
}

// Post is a single Buffer post (Buffer's API has historically called these
// "updates"). ScheduledAt is empty when the post is queued and Buffer will pick
// the slot; it carries an RFC3339 time when explicitly scheduled.
type Post struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channelId"`
	Text        string `json:"text"`
	Status      string `json:"status"`      // e.g. "draft", "queued", "scheduled", "sent"
	ScheduledAt string `json:"scheduledAt"` // RFC3339, empty when Buffer picks the slot
}

// CreatePostInput describes a post to create across one or more channels. The
// finished text is supplied by the caller — in the MCP case, the host model
// writes it; this layer never parses a free-form idea into a post.
type CreatePostInput struct {
	ChannelIDs []string // one or more Channel.ID values; required
	Text       string   // the finished post text; required
	When       string   // WhenQueue (default), or an RFC3339 timestamp to schedule
}

const listChannelsQuery = `query ListChannels {
  channels {
    id
    service
    name
  }
}`

// ListChannels returns the social channels connected to the token's Buffer org.
// Callers need these IDs before creating or listing posts, so this is normally
// the first call made.
func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var out struct {
		Channels []Channel `json:"channels"`
	}
	if err := c.Do(ctx, listChannelsQuery, nil, &out); err != nil {
		return nil, err
	}
	return out.Channels, nil
}

const createPostMutation = `mutation CreatePost($input: PostCreateInput!) {
  createPost(input: $input) {
    id
    channelId
    text
    status
    scheduledAt
  }
}`

// CreatePost creates a post on one or more channels. With When empty or
// WhenQueue, the post joins each channel's Buffer queue and Buffer chooses the
// time. With an RFC3339 When, the post is scheduled for that explicit time.
func (c *Client) CreatePost(ctx context.Context, in CreatePostInput) ([]Post, error) {
	if len(in.ChannelIDs) == 0 {
		return nil, fmt.Errorf("buffer: CreatePost requires at least one channel ID")
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, fmt.Errorf("buffer: CreatePost requires non-empty text")
	}

	input := map[string]any{
		"channelIds": in.ChannelIDs,
		"text":       in.Text,
	}
	switch {
	case in.When == "" || in.When == WhenQueue:
		input["schedule"] = WhenQueue // let Buffer pick the next open slot
	default:
		t, err := time.Parse(time.RFC3339, in.When)
		if err != nil {
			return nil, fmt.Errorf("buffer: CreatePost 'when' must be %q or an RFC3339 time, got %q: %w", WhenQueue, in.When, err)
		}
		input["scheduledAt"] = t.UTC().Format(time.RFC3339)
	}

	var out struct {
		CreatePost []Post `json:"createPost"`
	}
	if err := c.Do(ctx, createPostMutation, map[string]any{"input": input}, &out); err != nil {
		return nil, err
	}
	return out.CreatePost, nil
}

const listQueuedQuery = `query ListQueued($channelId: ID!) {
  queuedPosts(channelId: $channelId) {
    id
    channelId
    text
    status
    scheduledAt
  }
}`

// ListQueued returns the upcoming queued/scheduled posts for one channel, so a
// caller can see what's already planned before adding more (and space content
// out rather than double-posting).
func (c *Client) ListQueued(ctx context.Context, channelID string) ([]Post, error) {
	if strings.TrimSpace(channelID) == "" {
		return nil, fmt.Errorf("buffer: ListQueued requires a channel ID")
	}
	var out struct {
		QueuedPosts []Post `json:"queuedPosts"`
	}
	if err := c.Do(ctx, listQueuedQuery, map[string]any{"channelId": channelID}, &out); err != nil {
		return nil, err
	}
	return out.QueuedPosts, nil
}
