package handler

import (
	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/supabase"
)

type Handler struct {
	SB *supabase.Client
	AI *gemini.Client
}

func New(sb *supabase.Client, ai *gemini.Client) *Handler {
	return &Handler{SB: sb, AI: ai}
}
