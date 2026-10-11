package handler

import (
	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/supabase"
	"nutrily/backend/internal/turnstile"
)

type Handler struct {
	SB *supabase.Client
	AI *gemini.Client
	TS *turnstile.Client
}

func New(sb *supabase.Client, ai *gemini.Client, ts *turnstile.Client) *Handler {
	return &Handler{SB: sb, AI: ai, TS: ts}
}
