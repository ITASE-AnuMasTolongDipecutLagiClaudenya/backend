package gemini

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestText(t *testing.T) {
	got, err := Text([]byte(`{"candidates":[{"content":{"parts":[{"text":"halo "},{"text":"Nutrily"}]}}]}`))
	if err != nil || got != "halo Nutrily" {
		t.Fatalf("Text() = %q, %v", got, err)
	}
	if _, err := Text([]byte(`{"candidates":[]}`)); err == nil {
		t.Fatal("expected empty candidates to fail")
	}
}

func TestStripFence(t *testing.T) {
	got := StripFence("```json\n{\"items\": []}\n```")
	if got != "{\"items\": []}" {
		t.Fatalf("unexpected fence strip: %q", got)
	}
	got = StripFence("hasil:\n[{\"name\": \"Roti\"}] selesai")
	if got != "[{\"name\": \"Roti\"}]" {
		t.Fatalf("unexpected array strip: %q", got)
	}
}

func TestGenerateRetriesTransientProviderErrors(t *testing.T) {
	calls := 0
	c := Client{
		Model:  "test-model",
		APIKey: "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`
			status := http.StatusOK
			if calls < 3 {
				status = []int{http.StatusTooManyRequests, http.StatusServiceUnavailable}[calls-1]
				body = `{"error":{"status":"UNAVAILABLE"}}`
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
		})},
	}
	result, err := c.Generate(context.Background(), "", []any{}, false)
	if err != nil || result["answer"] != "ok" || calls != 3 {
		t.Fatalf("Generate() = %#v, %v after %d calls", result, err, calls)
	}
}
