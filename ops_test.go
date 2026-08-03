package buffer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockServer returns a Client pointed at an httptest server that replies with
// respBody and records the last decoded GraphQL request. No live token needed —
// this exercises the wiring (auth header, variables, response parsing), which is
// exactly what stays stable once the placeholder GraphQL documents are finalized.
func mockServer(t *testing.T, respBody string, capture *request) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		if capture != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, capture)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	return New("test-token", WithBaseURL(srv.URL))
}

func TestListChannels(t *testing.T) {
	c := mockServer(t, `{"data":{"channels":[
		{"id":"1","service":"twitter","name":"@acme"},
		{"id":"2","service":"linkedin","name":"Acme Inc"}
	]}}`, nil)

	chs, err := c.ListChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Fatalf("got %d channels, want 2", len(chs))
	}
	if chs[0].Service != "twitter" || chs[1].Name != "Acme Inc" {
		t.Fatalf("parsed channels wrong: %+v", chs)
	}
}

func TestCreatePostQueue(t *testing.T) {
	var req request
	c := mockServer(t, `{"data":{"createPost":[
		{"id":"9","channelId":"1","text":"hi","status":"queued"}
	]}}`, &req)

	posts, err := c.CreatePost(context.Background(), CreatePostInput{
		ChannelIDs: []string{"1"},
		Text:       "hi",
		When:       WhenQueue,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Status != "queued" {
		t.Fatalf("parsed posts wrong: %+v", posts)
	}
	input, ok := req.Variables["input"].(map[string]any)
	if !ok {
		t.Fatalf("input variable missing: %+v", req.Variables)
	}
	if input["schedule"] != WhenQueue {
		t.Errorf("schedule = %v, want %q", input["schedule"], WhenQueue)
	}
	if _, scheduled := input["scheduledAt"]; scheduled {
		t.Errorf("queued post should not set scheduledAt: %+v", input)
	}
}

func TestCreatePostScheduled(t *testing.T) {
	var req request
	c := mockServer(t, `{"data":{"createPost":[{"id":"9","channelId":"1","text":"hi","status":"scheduled"}]}}`, &req)

	_, err := c.CreatePost(context.Background(), CreatePostInput{
		ChannelIDs: []string{"1"},
		Text:       "hi",
		When:       "2030-01-02T15:04:05Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := req.Variables["input"].(map[string]any)
	if input["scheduledAt"] != "2030-01-02T15:04:05Z" {
		t.Errorf("scheduledAt = %v, want RFC3339 time", input["scheduledAt"])
	}
	if _, queued := input["schedule"]; queued {
		t.Errorf("scheduled post should not set schedule=queue: %+v", input)
	}
}

func TestCreatePostValidation(t *testing.T) {
	c := New("test-token") // never reached; validation fails first
	ctx := context.Background()

	if _, err := c.CreatePost(ctx, CreatePostInput{Text: "hi"}); err == nil {
		t.Error("expected error for missing channel IDs")
	}
	if _, err := c.CreatePost(ctx, CreatePostInput{ChannelIDs: []string{"1"}}); err == nil {
		t.Error("expected error for empty text")
	}
	if _, err := c.CreatePost(ctx, CreatePostInput{ChannelIDs: []string{"1"}, Text: "hi", When: "next tuesday"}); err == nil {
		t.Error("expected error for non-RFC3339 when")
	}
}

func TestListQueued(t *testing.T) {
	var req request
	c := mockServer(t, `{"data":{"queuedPosts":[{"id":"3","channelId":"1","text":"soon","status":"queued"}]}}`, &req)

	posts, err := c.ListQueued(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].ID != "3" {
		t.Fatalf("parsed posts wrong: %+v", posts)
	}
	if req.Variables["channelId"] != "1" {
		t.Errorf("channelId variable = %v, want 1", req.Variables["channelId"])
	}
	if _, err := c.ListQueued(context.Background(), ""); err == nil {
		t.Error("expected error for empty channel ID")
	}
}
