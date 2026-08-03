# buffer-go

Turn free-form ideas into real, scheduled [Buffer](https://buffer.com) posts —
from Claude, Cursor, or any MCP client.

`buffer-go` is two things stacked:

1. **A dependency-free Go client** for Buffer's GraphQL API (`buffer` package).
2. **An MCP server** (`cmd/buffer-mcp`) that exposes Buffer operations as tools
   an AI host can call.

The MCP server is deliberately thin: **the host model writes the post text and
decides what to do; the server just executes Buffer operations.** There's no
"idea parser" — the model turns "post something about our launch" into finished
text, and the server publishes it. That's what keeps it working with any MCP
client.

## Where it works

This is a **local (stdio) MCP server**: the AI app launches the `buffer-mcp`
binary as a subprocess on your machine and talks to it over stdin/stdout. You
don't host anything.

| Client | Works? | Notes |
|---|---|---|
| **Claude Code** (any OS, incl. Linux) | ✅ | `claude mcp add` — see below |
| **Claude Desktop** (macOS & Windows) | ✅ | config snippet below; ship a `.exe` for Windows |
| **Cursor / Zed / VS Code** | ✅ | any client that spawns local stdio servers |
| **claude.ai in the browser** | ❌ | a web page can't launch a local process → needs the hosted version (see [Roadmap](#roadmap)) |
| **Claude mobile app** | ❌ | same as web → hosted version |
| **Claude Code on the web / remote sessions** | ⚠️ | works only if the binary is installed *in that remote environment*, not on your laptop |

The rule: **anything local → works now. Anything in a browser or hosted → needs
the v2 hosted server.**

## Install

```bash
go install github.com/Trones21/buffer-go/cmd/buffer-mcp@latest
```

This puts `buffer-mcp` in your `$GOBIN` (usually `~/go/bin`). For Windows users,
build a `.exe` with `GOOS=windows go build ./cmd/buffer-mcp` and distribute that.

### Try it with no token (demo mode)

With no `BUFFER_TOKEN` set, the server runs in **demo mode** and returns canned
example data — so you can wire it into your client and watch the tools respond
before pasting a real token:

```bash
buffer-mcp --demo   # or just run with BUFFER_TOKEN unset
```

### Claude Code

```bash
claude mcp add buffer --env BUFFER_TOKEN=your_token_here -- buffer-mcp
```

### Claude Desktop

Add to `claude_desktop_config.json`
(macOS: `~/Library/Application Support/Claude/`, Windows: `%APPDATA%\Claude\`):

```json
{
  "mcpServers": {
    "buffer": {
      "command": "buffer-mcp",
      "env": { "BUFFER_TOKEN": "your_token_here" }
    }
  }
}
```

Restart the app. Now, when you ask Claude to "queue a post about X," it will call
the tools below.

## Tools

| Tool | What it does |
|---|---|
| `list_channels` | Lists connected channels (id, service, name). Called first — the others need channel IDs. |
| `create_post` | Creates a post from finished text on one or more channels. Adds to the Buffer queue by default; pass an RFC3339 `when` to schedule an explicit time. |
| `list_queued` | Lists upcoming queued/scheduled posts for a channel, so the model can space content out. |

### On "when to post"

You mostly don't schedule anything. Every Buffer channel already has a posting
schedule, and its **queue** fills those slots automatically. So `create_post`
defaults to `when: "queue"` — Buffer picks the time. Pass an explicit RFC3339
timestamp only when you want a specific slot. There is no scheduler in this
codebase, by design; Buffer is the scheduler.

## Using the library directly

Go programs can skip MCP and use the client. It's standard-library-only:

```bash
go get github.com/Trones21/buffer-go
```

```go
c, err := buffer.NewFromEnv()        // reads BUFFER_TOKEN
if err != nil { log.Fatal(err) }

channels, err := c.ListChannels(ctx)
posts, err := c.CreatePost(ctx, buffer.CreatePostInput{
    ChannelIDs: []string{channels[0].ID},
    Text:       "Shipping something new today.",
    When:       buffer.WhenQueue,   // or an RFC3339 timestamp
})
```

Low-level `Do(ctx, query, vars, out)` and `Introspect(ctx)` are still there for
anything the typed operations don't cover.

## Status & the introspection step

> **The GraphQL documents in `ops.go` are placeholders.** They're written
> against Buffer's documented concepts, not confirmed operation names. Demo mode
> works today; **live mode needs one step first**: run introspection with a real
> token to confirm Buffer's actual query/mutation names and input shapes, then
> replace the four `const` documents in `ops.go`. The Go types, method
> signatures, and MCP tools don't change when you do.

```go
c, _ := buffer.NewFromEnv()
s, _ := c.Introspect(ctx)
for _, f := range s.Mutations { fmt.Println(f.Name) }  // find the real names
```

## Architecture

```
buffer-go/
├── buffer.go, errors.go, introspect.go   # transport: auth, GraphQL, errors, introspection (zero deps)
├── ops.go                                 # typed operations: ListChannels, CreatePost, ListQueued
└── cmd/buffer-mcp/                        # the MCP server (own go.mod → SDK dep isolated here)
    └── main.go                            # stdio (default) + experimental --http
```

Three layers, each usable on its own: transport → typed operations → MCP server.
The server's `go.mod` is separate on purpose, so the MCP SDK dependency never
reaches consumers of the root library — `go get`-ing the library pulls **zero**
dependencies.

## Roadmap

- **Finalize the live GraphQL** via introspection (see above).
- **Hosted (v2):** the same server can run over streamable HTTP (`--http`,
  already wired but single-tenant/experimental). Turning that into a public,
  multi-tenant connector for claude.ai web and mobile is a real lift — not
  because of the transport, but because each caller needs their **own** Buffer
  auth (OAuth / per-connection tokens) instead of one shared `BUFFER_TOKEN`.
  The layered design means `ops.go` powers both without a rewrite.

## Auth

Set your Buffer personal access token as an environment variable — never commit
it:

```bash
export BUFFER_TOKEN="your_token_here"
```

## License

See [LICENSE](LICENSE).
