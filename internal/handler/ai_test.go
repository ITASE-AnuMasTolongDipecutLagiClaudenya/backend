package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/openrouter"
)

type aiRoundTripFunc func(*http.Request) (*http.Response, error)

func (f aiRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateAIFallsBackToOpenRouter(t *testing.T) {
	geminiClient := &gemini.Client{APIKey: "gemini-key", Model: "test-model", HTTP: &http.Client{Transport: aiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header), Request: r}, nil
	})}}
	openRouterClient := &openrouter.Client{APIKey: "openrouter-key", Model: "test-model", HTTP: &http.Client{Transport: aiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"fallback ok"}}]}`)), Header: make(http.Header), Request: r}, nil
	})}}
	h := &Handler{AI: geminiClient, OpenRouter: openRouterClient}
	result, err := h.GenerateAI(context.Background(), "", []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": "hello"}}}}, false)
	if err != nil || result["answer"] != "fallback ok" {
		t.Fatalf("GenerateAI() = %#v, %v", result, err)
	}
}
