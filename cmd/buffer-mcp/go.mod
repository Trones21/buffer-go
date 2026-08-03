// Separate module on purpose: the MCP SDK dependency lives here, so the root
// buffer library stays 100% dependency-free. The replace directive builds the
// server against the library in this repo.
module github.com/Trones21/buffer-go/cmd/buffer-mcp

go 1.25.0

require (
	github.com/Trones21/buffer-go v0.0.0
	github.com/modelcontextprotocol/go-sdk v1.7.0
)

require (
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)

replace github.com/Trones21/buffer-go => ../..
