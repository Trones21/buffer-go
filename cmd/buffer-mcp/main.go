// Command buffer-mcp is a Model Context Protocol (MCP) server that lets an MCP
// host (Claude Code, Claude Desktop, Cursor, …) turn free-form ideas into real
// Buffer posts. It is a thin adapter: the host model writes the post text and
// decides what to do; this server just exposes Buffer operations as tools and
// executes them. The "brain" is the model; this is the "hands".
//
// Transport:
//   - default: stdio. The MCP host launches this binary as a subprocess and
//     talks JSON-RPC over stdin/stdout. This is what you use locally.
//   - --http <addr>: streamable HTTP, single-tenant, experimental. See the
//     v2 boundary note in runHTTP and the Roadmap in the README.
//
// Auth: the Buffer personal access token is read from BUFFER_TOKEN. With no
// token set (or with --demo), the server runs in demo mode and returns canned
// example data, so you can wire it into a client and watch the tools respond
// before you ever paste a real token.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	buffer "github.com/Trones21/buffer-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "v0.1.0"

const demoNote = "demo mode: this is canned example data. Set BUFFER_TOKEN (and restart the server) to talk to the real Buffer API."

var demoChannels = []buffer.Channel{
	{ID: "demo-tw", Service: "twitter", Name: "@your_handle (demo)"},
	{ID: "demo-li", Service: "linkedin", Name: "Your Name (demo)"},
}

// app carries the shared state each tool handler needs.
type app struct {
	client *buffer.Client // nil in demo mode
	demo   bool
}

// --- tool: list_channels ---

type listChannelsInput struct{}

type listChannelsOutput struct {
	Channels []buffer.Channel `json:"channels"`
	Note     string           `json:"note,omitempty"`
}

func (a *app) listChannels(ctx context.Context, _ *mcp.CallToolRequest, _ listChannelsInput) (*mcp.CallToolResult, listChannelsOutput, error) {
	if a.demo {
		return nil, listChannelsOutput{Channels: demoChannels, Note: demoNote}, nil
	}
	chs, err := a.client.ListChannels(ctx)
	if err != nil {
		return nil, listChannelsOutput{}, err
	}
	return nil, listChannelsOutput{Channels: chs}, nil
}

// --- tool: create_post ---

type createPostInput struct {
	ChannelIDs []string `json:"channel_ids" jsonschema:"IDs of the channels to post to; obtain them from list_channels"`
	Text       string   `json:"text" jsonschema:"the finished post text to publish (you write this from the user's idea)"`
	When       string   `json:"when,omitempty" jsonschema:"'queue' (default) adds the post to the channel's Buffer queue and lets Buffer pick the time; or an RFC3339 timestamp like 2030-01-02T15:04:05Z to schedule an explicit time"`
}

type createPostOutput struct {
	Posts []buffer.Post `json:"posts"`
	Note  string        `json:"note,omitempty"`
}

func (a *app) createPost(ctx context.Context, _ *mcp.CallToolRequest, in createPostInput) (*mcp.CallToolResult, createPostOutput, error) {
	if a.demo {
		when := in.When
		if when == "" {
			when = buffer.WhenQueue
		}
		status, scheduledAt := "queued", ""
		if when != buffer.WhenQueue {
			status, scheduledAt = "scheduled", when
		}
		posts := make([]buffer.Post, 0, len(in.ChannelIDs))
		for _, id := range in.ChannelIDs {
			posts = append(posts, buffer.Post{ID: "demo-post", ChannelID: id, Text: in.Text, Status: status, ScheduledAt: scheduledAt})
		}
		return nil, createPostOutput{Posts: posts, Note: demoNote}, nil
	}
	posts, err := a.client.CreatePost(ctx, buffer.CreatePostInput{
		ChannelIDs: in.ChannelIDs,
		Text:       in.Text,
		When:       in.When,
	})
	if err != nil {
		return nil, createPostOutput{}, err
	}
	return nil, createPostOutput{Posts: posts}, nil
}

// --- tool: list_queued ---

type listQueuedInput struct {
	ChannelID string `json:"channel_id" jsonschema:"the channel ID whose upcoming queued/scheduled posts to list"`
}

type listQueuedOutput struct {
	Posts []buffer.Post `json:"posts"`
	Note  string        `json:"note,omitempty"`
}

func (a *app) listQueued(ctx context.Context, _ *mcp.CallToolRequest, in listQueuedInput) (*mcp.CallToolResult, listQueuedOutput, error) {
	if a.demo {
		return nil, listQueuedOutput{Posts: []buffer.Post{}, Note: demoNote}, nil
	}
	posts, err := a.client.ListQueued(ctx, in.ChannelID)
	if err != nil {
		return nil, listQueuedOutput{}, err
	}
	return nil, listQueuedOutput{Posts: posts}, nil
}

func main() {
	httpAddr := flag.String("http", "", "serve over streamable HTTP on this address (e.g. :8080) instead of stdio; single-tenant, experimental — see README Roadmap")
	demo := flag.Bool("demo", false, "force demo mode: serve canned example data without calling Buffer")
	flag.Parse()

	token := strings.TrimSpace(os.Getenv(buffer.TokenEnv))
	a := &app{demo: *demo || token == ""}
	if a.demo {
		log.Printf("buffer-mcp %s: demo mode (no %s set) — serving example data", version, buffer.TokenEnv)
	} else {
		a.client = buffer.New(token)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "buffer", Version: version}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_channels",
		Description: "List the social channels connected to the Buffer account (id, service, name). Call this first — the other tools need channel IDs.",
	}, a.listChannels)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_post",
		Description: "Create a Buffer post. Provide the finished post text and one or more channel IDs. By default the post is added to the channel's Buffer queue and Buffer picks the time; pass an RFC3339 timestamp in 'when' to schedule an explicit time.",
	}, a.createPost)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_queued",
		Description: "List the upcoming queued/scheduled posts for a channel, so you can see what's already planned before adding more.",
	}, a.listQueued)

	if *httpAddr != "" {
		runHTTP(*httpAddr, server)
		return
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("buffer-mcp: %v", err)
	}
}

// runHTTP serves the MCP server over streamable HTTP.
//
// v2 boundary: this hands the SAME single BUFFER_TOKEN to every caller. That is
// fine for a self-hosted, single-user deployment, but is NOT safe as a public
// multi-tenant service — every caller would post as you. Real multi-tenant
// hosting needs per-caller auth: wrap the handler with mcp middleware such as
// RequireBearerToken and make the getServer func below build a Buffer client
// from the authenticated caller's own token/identity. The transport is already
// here; that auth work is the actual v2 lift.
func runHTTP(addr string, server *mcp.Server) {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	log.Printf("buffer-mcp %s: streamable HTTP on %s (single-tenant, experimental)", version, addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("buffer-mcp: %v", err)
	}
}
