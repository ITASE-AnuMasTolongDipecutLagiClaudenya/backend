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

func TestDeleteAppliesFiltersToURL(t *testing.T) {
	client := &Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodDelete || request.URL.Query().Get("user_id") != "eq.user-1" || request.URL.Query().Get("conversation_id") != "eq.550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("DELETE URL missing filters: %s", request.URL.String())
			}
			if request.Body != nil && request.Body != http.NoBody {
				t.Error("DELETE unexpectedly has a body")
			}
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: request}, nil
		})},
	}
	if _, err := client.Query(context.Background(), "Bearer token", http.MethodDelete, "chat_messages", "user_id=eq.user-1&conversation_id=eq.550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatalf("DELETE filtered query failed: %v", err)
	}
}

func TestGetAppliesFiltersToURL(t *testing.T) {
	client := &Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Query().Get("user_id") != "eq.user-1" {
				t.Errorf("GET URL missing filters: %s", request.URL.String())
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header), Request: request}, nil
		})},
	}
	if _, err := client.Query(context.Background(), "Bearer token", http.MethodGet, "profiles", "user_id=eq.user-1&select=*"); err != nil {
		t.Fatalf("GET filtered query failed: %v", err)
	}
}
