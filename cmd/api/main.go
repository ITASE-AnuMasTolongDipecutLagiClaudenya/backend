package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"nutrily/backend/internal/config"
	"nutrily/backend/internal/exa"
	"nutrily/backend/internal/gemini"
	"nutrily/backend/internal/groq"
	"nutrily/backend/internal/handler"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/openai"
	"nutrily/backend/internal/scanbudget"
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
	geminiHTTPClient := &http.Client{Timeout: 55 * time.Second}
	groqHTTPClient := &http.Client{Timeout: 2 * time.Minute}
	openaiHTTPClient := &http.Client{Timeout: 60 * time.Second}
	exaHTTPClient := &http.Client{Timeout: 30 * time.Second}
	sb := &supabase.Client{BaseURL: cfg.SupabaseURL, AnonKey: cfg.AnonKey, HTTP: httpClient}
	ai := &gemini.Client{APIKeys: cfg.GeminiKeys, Model: cfg.GeminiModel, HTTP: geminiHTTPClient}
	groqAI := &groq.Client{APIKeys: cfg.GroqKeys, Model: cfg.GroqModel, HTTP: groqHTTPClient}
	openaiAI := &openai.Client{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, HTTP: openaiHTTPClient}
	exaAI := &exa.Client{APIKey: cfg.ExaKey, HTTP: exaHTTPClient, NumResults: cfg.ExaNumResults, TrustedDomains: cfg.ExaTrustedDomains}
	ts := &turnstile.Client{Secret: cfg.TurnstileSecret, HTTP: httpClient}
	apiHandler := handler.New(sb, ai, groqAI, openaiAI, exaAI, ts, catalog, cfg.CloudinaryCloud, cfg.CloudinaryKey, cfg.CloudinarySecret)
	apiHandler.Store = &handler.Store{
		Cache:        scanbudget.NewCache(cfg.ProductCacheMaxAge),
		Quota:        scanbudget.NewQuota(cfg.ScanDailyQuota, cfg.ScanMonthlyQuota),
		SearchMaxRes: cfg.SearchMaxResults,
	}
	mux := server.New(cfg, sb, apiHandler)
	addr := ":" + cfg.Port
	log.Printf("Nutrily API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
