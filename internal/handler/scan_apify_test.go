package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunApifyOCR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var input struct {
			SearchTypes  []string `json:"searchTypes"`
			ImagesBase64 []string `json:"imagesBase64"`
			Language     string   `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("decode input: %v", err)
		}
		if len(input.SearchTypes) != 1 || input.SearchTypes[0] != "ocr" || input.Language != "id" {
			t.Errorf("unexpected Apify input: %#v", input)
		}
		if len(input.ImagesBase64) != 1 || !strings.HasSuffix(input.ImagesBase64[0], base64.StdEncoding.EncodeToString([]byte("image"))) {
			t.Errorf("imagesBase64 = %#v", input.ImagesBase64)
		}
		_, _ = io.WriteString(w, `[{"searchType":"ocr","ocr":{"originalExtractedText":"  Label text  "}}]`)
	}))
	defer server.Close()

	text, err := runApifyOCR(context.Background(), server.Client(), server.URL, "test-token", []byte("image"), "image/jpeg")
	if err != nil {
		t.Fatalf("runApifyOCR() error = %v", err)
	}
	if text != "Label text" {
		t.Fatalf("runApifyOCR() = %q", text)
	}
}

func TestEstimatePromptUsesOCRTextOnly(t *testing.T) {
	prompt := estimatePrompt([]map[string]any{{"name": "Susu"}}, "label", "Energi 120 kkal")
	if !strings.Contains(prompt, "Energi 120 kkal") || !strings.Contains(prompt, "Item") {
		t.Fatalf("estimatePrompt() omitted OCR text or items: %q", prompt)
	}
}
