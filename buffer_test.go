package buffer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDo_SetsBearerAndDecodesData(t *testing.T) {
	var gotAuth, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"data":{"viewer":{"name":"Acme"}}}`))
	}))
	defer srv.Close()

	c := New("tok_123", WithBaseURL(srv.URL))
	var out struct {
		Viewer struct{ Name string } `json:"viewer"`
	}
	if err := c.Do(context.Background(), `query { viewer { name } }`, map[string]any{"x": 1}, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer tok_123" {
		t.Errorf("Authorization header: got %q", gotAuth)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("Content-Type: got %q", gotContentType)
	}
	if !strings.Contains(gotBody, `"query"`) || !strings.Contains(gotBody, `"variables"`) {
		t.Errorf("body missing query/variables: %s", gotBody)
	}
	if out.Viewer.Name != "Acme" {
		t.Errorf("decoded data: got %q, want Acme", out.Viewer.Name)
	}
}

func TestDo_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := New("bad", WithBaseURL(srv.URL))
	err := c.Do(context.Background(), `query { viewer { name } }`, nil, nil)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDo_GraphQLErrorsSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"Cannot query field \"nope\""}]}`))
	}))
	defer srv.Close()

	c := New("tok", WithBaseURL(srv.URL))
	err := c.Do(context.Background(), `query { nope }`, nil, nil)
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) {
		t.Fatalf("expected *GraphQLError, got %v", err)
	}
	if len(gqlErr.Messages) != 1 || !strings.Contains(gqlErr.Messages[0], "nope") {
		t.Errorf("unexpected messages: %v", gqlErr.Messages)
	}
}

func TestDo_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer srv.Close()

	c := New("tok", WithBaseURL(srv.URL))
	err := c.Do(context.Background(), `query { x }`, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %v", err)
	}
	if httpErr.Status != http.StatusInternalServerError {
		t.Errorf("status: got %d", httpErr.Status)
	}
}

func TestNewFromEnv_RequiresToken(t *testing.T) {
	t.Setenv(TokenEnv, "")
	if _, err := NewFromEnv(); err == nil {
		t.Fatal("expected an error when BUFFER_TOKEN is unset")
	}
	t.Setenv(TokenEnv, "tok_env")
	c, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	if c.token != "tok_env" {
		t.Errorf("token not read from env: %q", c.token)
	}
}

func TestIntrospect_ParsesOperationNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Confirm the introspection query was actually sent.
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "__schema") {
			t.Errorf("not an introspection query: %s", b)
		}
		resp := map[string]any{"data": map[string]any{"__schema": map[string]any{
			"queryType":    map[string]any{"fields": []map[string]string{{"name": "channels"}, {"name": "posts"}}},
			"mutationType": map[string]any{"fields": []map[string]string{{"name": "createPost"}}},
		}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New("tok", WithBaseURL(srv.URL))
	s, err := c.Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	if len(s.Queries) != 2 || s.Queries[0].Name != "channels" {
		t.Errorf("queries: %+v", s.Queries)
	}
	if len(s.Mutations) != 1 || s.Mutations[0].Name != "createPost" {
		t.Errorf("mutations: %+v", s.Mutations)
	}
}
