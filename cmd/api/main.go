package main

import (
	"log"
	"net/http"
	"time"

	"nutrily/backend/internal/config"
	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/handler"
	"nutrily/backend/internal/server"
	"nutrily/backend/internal/supabase"
	"nutrily/backend/internal/turnstile"
)

func main() {
	cfg := config.Load()
	if cfg.SupabaseURL == "" || cfg.AnonKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_ANON_KEY are required")
	}
	httpClient := &http.Client{Timeout: 45 * time.Second}
	sb := &supabase.Client{BaseURL: cfg.SupabaseURL, AnonKey: cfg.AnonKey, HTTP: httpClient}
	ai := &gemini.Client{APIKey: cfg.GeminiKey, Model: cfg.Model, HTTP: httpClient}
	ts := &turnstile.Client{Secret: cfg.TurnstileSecret, HTTP: httpClient}
	mux := server.New(cfg, sb, handler.New(sb, ai, ts))
	addr := ":" + cfg.Port
	log.Printf("Nutrily API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
