package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateConvertsGeminiVisionAndJSON(t *testing.T) {
	data := "aGVsbG8="
	var got map[string]any
	client := Client{
		APIKey: "test-key",
		Model:  "google/gemma-4-26b-a4b-it:free",
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			body := `{"choices":[{"message":{"content":"{\"items\":[{\"name\":\"Apel\"}]}"}}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
		})},
	}
	contents := []any{map[string]any{"role": "user", "parts": []any{
		map[string]string{"text": "Baca item"},
		map[string]any{"inlineData": map[string]string{"mimeType": "image/jpeg", "data": data}},
	}}}
	result, err := client.Generate(context.Background(), "", contents, true)
	if err != nil || result["items"] == nil {
		t.Fatalf("Generate() = %#v, %v", result, err)
	}
	messages := got["messages"].([]any)
	parts := messages[0].(map[string]any)["content"].([]any)
	if parts[0].(map[string]any)["text"] != "Baca item" || parts[1].(map[string]any)["image_url"].(map[string]any)["url"] != "data:image/jpeg;base64,"+data {
		t.Fatalf("unexpected OpenRouter multimodal payload: %#v", parts)
	}
	if got["response_format"].(map[string]any)["type"] != "json_object" {
		t.Fatalf("response_format = %#v", got["response_format"])
	}
}
