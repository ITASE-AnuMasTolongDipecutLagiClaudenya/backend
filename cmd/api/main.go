package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"nutrily/backend/internal/config"
	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/handler"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/openrouter"
	"nutrily/backend/internal/server"
	"nutrily/backend/internal/supabase"
	"nutrily/backend/internal/turnstile"
)

func main() {
	cfg := config.Load()
	if cfg.SupabaseURL == "" || cfg.AnonKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_ANON_KEY are required")
	}
	catalogFile, err := os.Open("data/gizi_lookup.csv")
	if err != nil {
		log.Fatalf("open nutrition catalog: %v", err)
	}
	catalog, err := nutrition.LoadCatalog(catalogFile)
	closeErr := catalogFile.Close()
	if err != nil {
		log.Fatalf("load nutrition catalog: %v", err)
	}
	if closeErr != nil {
		log.Fatalf("close nutrition catalog: %v", closeErr)
	}
	httpClient := &http.Client{Timeout: 45 * time.Second}
	geminiHTTPClient := &http.Client{Timeout: 35 * time.Second}
	openRouterHTTPClient := &http.Client{Timeout: 2 * time.Minute}
	sb := &supabase.Client{BaseURL: cfg.SupabaseURL, AnonKey: cfg.AnonKey, HTTP: httpClient}
	ai := &gemini.Client{APIKey: cfg.GeminiKey, Model: cfg.Model, HTTP: geminiHTTPClient}
	openRouterAI := &openrouter.Client{APIKey: cfg.OpenRouterKey, Model: cfg.OpenRouterModel, HTTP: openRouterHTTPClient}
	ts := &turnstile.Client{Secret: cfg.TurnstileSecret, HTTP: httpClient}
	apiHandler := handler.New(sb, ai, openRouterAI, ts, catalog, cfg.CloudinaryCloud, cfg.CloudinaryKey, cfg.CloudinarySecret)
	apiHandler.ApifyToken = cfg.ApifyToken
	mux := server.New(cfg, sb, apiHandler)
	addr := ":" + cfg.Port
	log.Printf("Nutrily API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
