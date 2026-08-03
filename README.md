# buffer-go

An adapter for getting free-form ideas/content published and scheduled through
the [Buffer API](https://developers.buffer.com).

> **Status: early — the shape is undecided.** This may end up a Go module, a CLI,
> or an MCP server; that call hasn't been made. Don't read the current package
> layout as a commitment to "it's a library." What exists today is the
> irreducible starting point that any of those shapes would need: a tiny,
> dependency-free client for Buffer's GraphQL API (Bearer-token auth) plus schema
> introspection.

## What it's for

Turn a loosely-structured idea ("schedule a post about X") into a real Buffer
post via the API — the adapter between free-form input and Buffer's operations.
How that input arrives (a function call, a CLI invocation, an MCP tool call) is
exactly the part still being decided.

## What's here now

A dependency-free client for Buffer's single GraphQL endpoint
(`https://api.buffer.com`), authenticated with a personal access token sent as a
**Bearer token**:

- `buffer.Client` with `New` / `NewFromEnv` (reads `BUFFER_TOKEN`) and
  `Do(ctx, query, vars, out)`.
- Typed errors: `buffer.ErrUnauthorized` (401), `*buffer.HTTPError` (other
  non-2xx), `*buffer.GraphQLError` (GraphQL errors on a 200).
- `Introspect(ctx)` to read the live schema's operations — useful for building
  against real names without an SDK.

```go
c, err := buffer.NewFromEnv()          // reads BUFFER_TOKEN
if err != nil { log.Fatal(err) }

s, _ := c.Introspect(ctx)              // discover the real operations
for _, f := range s.Mutations { fmt.Println(f.Name) }
```

## Auth

Set your token as an environment variable — never commit it:

```bash
export BUFFER_TOKEN="your_token_here"
```

## Not yet decided

- **Delivery shape** — module vs. CLI vs. MCP server.
- **Typed operations** — create post, list channels, schedule with `dueAt`; added
  once the schema is confirmed. Until then `Do` + `Introspect` cover everything.
- **How free-form input is parsed** into a concrete Buffer post.
