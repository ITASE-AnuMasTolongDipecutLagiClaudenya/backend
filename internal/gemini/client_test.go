package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func TestGenerateRetriesNetworkErrorWithNextAPIKey(t *testing.T) {
	keys := []string{"key-1", "key-2"}
	var seen []string
	calls := 0
	c := Client{APIKeys: keys, Model: "test-model", HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		seen = append(seen, r.Header.Get("x-goog-api-key"))
		if calls == 1 {
			return nil, context.DeadlineExceeded
		}
		body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})}}
	result, err := c.Generate(context.Background(), "", []any{}, false)
	if err != nil || result["answer"] != "ok" || calls != 2 {
		t.Fatalf("Generate() = %#v, %v after %d calls", result, err, calls)
	}
	if seen[0] != keys[0] || seen[1] != keys[1] {
		t.Fatalf("API keys used = %v, want rotation %v", seen, keys)
	}
}

func TestGenerateImage25FlashRotatesKeysWithoutThinkingConfig(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3", "key-4", "key-5", "key-6"}
	seen := make([]string, 0, len(keys))
	c := Client{APIKeys: keys, Model: "gemini-2.5-flash", HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.Header.Get("x-goog-api-key"))
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		generation := payload["generationConfig"].(map[string]any)
		if _, exists := generation["thinkingConfig"]; exists {
			t.Errorf("unexpected thinkingConfig: %#v", generation["thinkingConfig"])
		}
		contents := payload["contents"].([]any)
		parts := contents[0].(map[string]any)["parts"].([]any)
		inline := parts[1].(map[string]any)["inlineData"].(map[string]any)
		if inline["mimeType"] != "image/jpeg" || inline["data"] != base64.StdEncoding.EncodeToString([]byte("image")) {
			t.Errorf("inline image = %#v", inline)
		}
		body := `{"candidates":[{"content":{"parts":[{"text":"{\"items\":[]}"}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})}}
	for range keys {
		if _, err := c.GenerateImage(context.Background(), "scan", []byte("image"), "image/jpeg", true); err != nil {
			t.Fatal(err)
		}
	}
	for i, key := range keys {
		if seen[i] != key {
			t.Fatalf("key %d = %q, want %q", i, seen[i], key)
		}
	}
}

func TestGenerate25FlashOmitsThinkingConfig(t *testing.T) {
	c := Client{APIKey: "test-key", Model: "gemini-2.5-flash", HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		generation := payload["generationConfig"].(map[string]any)
		if _, exists := generation["thinkingConfig"]; exists {
			t.Errorf("unexpected thinkingConfig: %#v", generation["thinkingConfig"])
		}
		body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})}}
	if _, err := c.Generate(context.Background(), "", []any{}, false); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateGroundedUsesSearchAndVerifiedSources(t *testing.T) {
	c := Client{APIKey: "test-key", Model: "gemini-2.5-flash", HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		tools := payload["tools"].([]any)
		if _, ok := tools[0].(map[string]any)["google_search"]; !ok {
			t.Fatalf("Google Search tool missing: %#v", tools)
		}
		body := `{"candidates":[{"content":{"parts":[{"text":"{\"items\":[{\"name\":\"Susu\",\"calories\":120}]}"}]},"groundingMetadata":{"groundingChunks":[{"web":{"uri":"https://example.com/nutrition","title":"Nutrition facts"}},{"web":{"uri":"http://insecure.example/","title":"Ignored"}}]}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})}}
	result, err := c.GenerateGrounded(context.Background(), "estimate")
	if err != nil {
		t.Fatal(err)
	}
	item := result["items"].([]any)[0].(map[string]any)
	sources := item["sources"].([]map[string]string)
	if len(sources) != 1 || sources[0]["url"] != "https://example.com/nutrition" {
		t.Fatalf("sources = %#v", sources)
	}
}
