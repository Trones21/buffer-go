package buffer

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnauthorized is returned when Buffer responds 401 — almost always a
// missing, wrong, or expired BUFFER_TOKEN.
var ErrUnauthorized = errors.New("buffer: unauthorized (401) — check BUFFER_TOKEN")

// HTTPError is a non-2xx response other than 401.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("buffer: http %d", e.Status)
	}
	return fmt.Sprintf("buffer: http %d: %s", e.Status, e.Body)
}

// GraphQLError carries the `errors` array Buffer returns on a 200 when a query
// or mutation is itself invalid (bad field, permission, validation).
type GraphQLError struct {
	Messages []string
}

func (e *GraphQLError) Error() string {
	return "buffer: graphql error: " + strings.Join(e.Messages, "; ")
}
