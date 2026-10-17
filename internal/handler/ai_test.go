package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"nutrily/backend/internal/groq"
)

type aiRoundTripFunc func(*http.Request) (*http.Response, error)

func (f aiRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateAIUsesGroq(t *testing.T) {
	groqClient := &groq.Client{APIKey: "groq-key", Model: "test-model", HTTP: &http.Client{Transport: aiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"fallback ok"}}]}`)), Header: make(http.Header), Request: r}, nil
	})}}
	h := &Handler{Groq: groqClient}
	result, err := h.GenerateAI(context.Background(), "", []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": "hello"}}}}, false)
	if err != nil || result["answer"] != "fallback ok" {
		t.Fatalf("GenerateAI() = %#v, %v", result, err)
	}
}
