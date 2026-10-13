package supabase

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDeleteUsesMinimalReturn(t *testing.T) {
	client := &Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if got := request.Header.Get("Prefer"); got != "return=minimal" {
				t.Errorf("Prefer = %q, want return=minimal", got)
			}
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})},
	}
	if _, err := client.Query(context.Background(), "Bearer token", http.MethodDelete, "chat_messages?user_id=eq.user"); err != nil {
		t.Fatalf("DELETE query failed: %v", err)
	}
}
