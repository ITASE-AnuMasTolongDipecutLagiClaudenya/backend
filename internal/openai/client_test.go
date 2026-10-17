package openai

import (
	"encoding/json"
	"testing"
)

func TestParseJSONTextObject(t *testing.T) {
	got, err := parseJSONText(`{"items":[{"name":"Susu"}]}`)
	if err != nil || got["items"] == nil {
		t.Fatalf("parseJSONText object = %#v, %v", got, err)
	}
}

func TestParseJSONTextFenced(t *testing.T) {
	got, err := parseJSONText("```json\n{\"items\":[]}\n```")
	if err != nil || got["items"] == nil {
		t.Fatalf("fenced JSON should parse: %#v, %v", got, err)
	}
}

func TestParseJSONTextList(t *testing.T) {
	got, err := parseJSONText(`[{"name":"Susu"}]`)
	if err != nil || got["items"] == nil {
		t.Fatalf("list should wrap into items: %#v, %v", got, err)
	}
}

func TestParseJSONTextInvalid(t *testing.T) {
	if _, err := parseJSONText("this is not json"); err == nil {
		t.Fatal("invalid JSON must return an error (so callers do not retry)")
	}
}

func TestParseResponsesExtractsSources(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"output": []any{
			map[string]any{"type": "web_search_call"},
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": `{"items":[{"name":"Susu","calories":120}]}`,
						"annotations": []any{
							map[string]any{"type": "url_citation", "url": "https://example.com/nutrition", "title": "Nutrition facts"},
						},
					},
				},
			},
		},
	})
	got, err := parseResponses(payload)
	if err != nil {
		t.Fatalf("parseResponses = %v", err)
	}
	sources, ok := got["_search_sources"].([]map[string]string)
	if !ok || len(sources) != 1 || sources[0]["url"] != "https://example.com/nutrition" {
		t.Fatalf("expected one extracted source, got %#v", got["_search_sources"])
	}
}
