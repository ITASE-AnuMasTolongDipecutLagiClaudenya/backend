package openai

import (
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

func TestExtractJSONWithProsePreambleAndFence(t *testing.T) {
	text := "Berikut adalah format JSON:\n\n```json\n{\"items\":[{\"name\":\"KANZLR\"}]}\n```\nSemoga membantu."
	got, err := parseJSONText(text)
	if err != nil {
		t.Fatalf("parseJSONText with prose+fence = %v", err)
	}
	if got["items"] == nil {
		t.Fatalf("expected items, got %#v", got)
	}
}

func TestExtractJSONWithLeadingProseOnly(t *testing.T) {
	text := `Tentu, ini hasilnya: {"items":[{"name":"Teh"}]} selesai.`
	got, err := parseJSONText(text)
	if err != nil || got["items"] == nil {
		t.Fatalf("parseJSONText with prose = %#v, %v", got, err)
	}
}
