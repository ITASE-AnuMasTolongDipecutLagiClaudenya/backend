package handler

import (
	"context"
	"errors"

	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/groq"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/supabase"
	"nutrily/backend/internal/turnstile"
)

var errNoAIProvider = errors.New("tidak ada provider AI aktif; isi GROQ_API_KEYS")

type Handler struct {
	SB               *supabase.Client
	Gemini           *gemini.Client
	Groq             *groq.Client
	TS               *turnstile.Client
	Catalog          *nutrition.Catalog
	CloudinaryCloud  string
	CloudinaryKey    string
	CloudinarySecret string
}

func New(sb *supabase.Client, ai *gemini.Client, groqAI *groq.Client, ts *turnstile.Client, catalog *nutrition.Catalog, cloud, cloudKey, cloudSecret string) *Handler {
	return &Handler{SB: sb, Gemini: ai, Groq: groqAI, TS: ts, Catalog: catalog, CloudinaryCloud: cloud, CloudinaryKey: cloudKey, CloudinarySecret: cloudSecret}
}

func (h *Handler) GenerateAI(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	if h.HasAI() {
		return h.Groq.Generate(ctx, systemInstruction, contents, jsonMode)
	}
	return nil, errNoAIProvider
}

func (h *Handler) HasAI() bool {
	return h.Groq != nil && (len(h.Groq.APIKeys) > 0 || h.Groq.APIKey != "")
}

func (h *Handler) HasGemini() bool {
	return h.Gemini != nil && (len(h.Gemini.APIKeys) > 0 || h.Gemini.APIKey != "")
}
