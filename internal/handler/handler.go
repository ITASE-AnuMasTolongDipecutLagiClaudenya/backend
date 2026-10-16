package handler

import (
	"context"
	"errors"
	"log"

	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/openrouter"
	"nutrily/backend/internal/supabase"
	"nutrily/backend/internal/turnstile"
)

var errNoAIProvider = errors.New("tidak ada provider AI aktif; isi GEMINI_API_KEY atau OPENROUTER_API_KEY")

type Handler struct {
	SB               *supabase.Client
	AI               *gemini.Client
	OpenRouter       *openrouter.Client
	TS               *turnstile.Client
	Catalog          *nutrition.Catalog
	CloudinaryCloud  string
	CloudinaryKey    string
	CloudinarySecret string
	ApifyToken       string
}

func New(sb *supabase.Client, ai *gemini.Client, openRouter *openrouter.Client, ts *turnstile.Client, catalog *nutrition.Catalog, cloud, cloudKey, cloudSecret string) *Handler {
	return &Handler{SB: sb, AI: ai, OpenRouter: openRouter, TS: ts, Catalog: catalog, CloudinaryCloud: cloud, CloudinaryKey: cloudKey, CloudinarySecret: cloudSecret}
}

func (h *Handler) GenerateAI(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	if h.AI != nil && h.AI.APIKey != "" {
		result, err := h.AI.Generate(ctx, systemInstruction, contents, jsonMode)
		if err == nil {
			return result, nil
		}
		if h.OpenRouter == nil || h.OpenRouter.APIKey == "" {
			return nil, err
		}
		log.Printf("Gemini failed; falling back to OpenRouter: %v", err)
	}
	if h.OpenRouter != nil && h.OpenRouter.APIKey != "" {
		return h.OpenRouter.Generate(ctx, systemInstruction, contents, jsonMode)
	}
	return nil, errNoAIProvider
}

func (h *Handler) HasAI() bool {
	return (h.AI != nil && h.AI.APIKey != "") || (h.OpenRouter != nil && h.OpenRouter.APIKey != "")
}
