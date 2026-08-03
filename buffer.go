// Package buffer is a small, reusable client for the Buffer API.
//
// Buffer exposes a single GraphQL endpoint (https://api.buffer.com); every
// request authenticates with a personal access token sent as a Bearer token.
// This package owns exactly that irreducible mechanics layer — auth, the GraphQL
// transport, error mapping, and schema introspection — and nothing project-
// specific. Each consuming project wraps it with its own content/scheduling
// policy (what to post, when, from which data).
//
// It is intentionally dependency-free (standard library only) so it drops into
// any Go project via `go get github.com/Trones21/buffer-go`.
//
//	c, err := buffer.NewFromEnv()          // reads BUFFER_TOKEN
//	if err != nil { ... }
//	var out struct{ ... }
//	err = c.Do(ctx, `query { ... }`, nil, &out)
package buffer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is Buffer's GraphQL endpoint.
const DefaultBaseURL = "https://api.buffer.com"

// TokenEnv is the environment variable NewFromEnv reads the Bearer token from.
const TokenEnv = "BUFFER_TOKEN"

// Client is a Buffer GraphQL client. Safe for concurrent use; create one and
// share it.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API endpoint (useful for tests or a mock).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient supplies a custom *http.Client (timeouts, transport, proxy).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New builds a Client from an explicit token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewFromEnv builds a Client from the BUFFER_TOKEN environment variable. It
// errors if the variable is unset or empty, so a missing credential fails fast
// and locally rather than as a 401 mid-flight.
func NewFromEnv(opts ...Option) (*Client, error) {
	token := strings.TrimSpace(os.Getenv(TokenEnv))
	if token == "" {
		return nil, fmt.Errorf("buffer: %s is not set", TokenEnv)
	}
	return New(token, opts...), nil
}

// request is the GraphQL request envelope.
type request struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// response is the GraphQL response envelope. Data is captured raw so Do can
// unmarshal it into the caller's typed target.
type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type gqlError struct {
	Message string `json:"message"`
}

// Do executes a GraphQL query or mutation (Buffer uses one endpoint for both).
// It sets the Bearer token, posts the request, and — on success — unmarshals the
// `data` field into out (pass nil to ignore the body).
//
// Errors are typed so callers can branch: ErrUnauthorized on a 401,
// *HTTPError on any other non-2xx, *GraphQLError when the response carries
// GraphQL errors.
func (c *Client) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(request{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("buffer: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("buffer: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("buffer: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("buffer: read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}

	var parsed response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("buffer: decode response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, len(parsed.Errors))
		for i, e := range parsed.Errors {
			msgs[i] = e.Message
		}
		return &GraphQLError{Messages: msgs}
	}
	if out != nil && len(parsed.Data) > 0 {
		if err := json.Unmarshal(parsed.Data, out); err != nil {
			return fmt.Errorf("buffer: decode data: %w", err)
		}
	}
	return nil
}
