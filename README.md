# buffer-go

A small, dependency-free Go client for the [Buffer API](https://developers.buffer.com).

Buffer exposes a single GraphQL endpoint (`https://api.buffer.com`) and
authenticates with a personal access token sent as a **Bearer token**. This
module owns just that mechanics layer — auth, the GraphQL transport, typed
errors, and schema introspection. Each project wraps it with its own
content/scheduling policy (what to post, when, from which data).

## Install

```bash
go get github.com/Trones21/buffer-go
```

## Auth

Set your token as an environment variable — never commit it:

```bash
export BUFFER_TOKEN="your_token_here"
```

## Usage

```go
c, err := buffer.NewFromEnv()          // reads BUFFER_TOKEN
if err != nil {
    log.Fatal(err)
}

var out struct {
    Viewer struct{ Name string } `json:"viewer"`
}
err = c.Do(ctx, `query { viewer { name } }`, nil, &out)
```

Errors are typed so callers can branch: `buffer.ErrUnauthorized` (401),
`*buffer.HTTPError` (other non-2xx), `*buffer.GraphQLError` (GraphQL errors on a
200).

## Discovering the schema

Buffer's operation names/fields are documented at developers.buffer.com, but you
can also read them straight from the API — GraphQL is introspectable:

```go
s, _ := c.Introspect(ctx)
for _, f := range s.Mutations {
    fmt.Println(f.Name, "—", f.Description)
}
```

## Scope

This is intentionally the **constant** part — the API mechanics that don't vary
by project. Typed operation helpers (create post, list channels, …) are added as
the schema is confirmed; until then, `Do` + `Introspect` cover everything.
