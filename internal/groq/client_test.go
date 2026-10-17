package groq

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

func TestGenerateJSONAndDisablesReasoning(t *testing.T) {
	var got map[string]any
	client := Client{
		APIKey: "test-key",
		Model:  "openai/gpt-oss-20b",
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != endpoint || r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			body := `{"choices":[{"message":{"content":"{\"items\":[{\"name\":\"Apel\"}]}"}}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
		})},
	}
	contents := []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": "Baca item"}}}}
	result, err := client.Generate(context.Background(), "", contents, true)
	if err != nil || result["items"] == nil {
		t.Fatalf("Generate() = %#v, %v", result, err)
	}
	messages := got["messages"].([]any)
	if messages[0].(map[string]any)["content"] != "Baca item" {
		t.Fatalf("messages = %#v", messages)
	}
	if got["response_format"].(map[string]any)["type"] != "json_object" {
		t.Fatalf("response_format = %#v", got["response_format"])
	}
	if got["include_reasoning"] != false {
		t.Fatalf("reasoning settings = %#v", got)
	}
}

func TestAPIKeyRotation(t *testing.T) {
	c := Client{APIKeys: []string{"one", "two", "three", "four", "five", "six"}}
	for _, want := range c.APIKeys {
		if got := c.nextAPIKey(); got != want {
			t.Fatalf("nextAPIKey() = %q, want %q", got, want)
		}
	}
}
