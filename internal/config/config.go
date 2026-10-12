package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SupabaseURL     string
	AnonKey         string
	GeminiKey       string
	Model           string
	OpenRouterKey   string
	OpenRouterModel string
	Port            string
	TurnstileSecret string
	Origins         map[string]bool
}

func Load() Config {
	loadEnv()
	c := Config{
		SupabaseURL:     strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		AnonKey:         os.Getenv("SUPABASE_ANON_KEY"),
		GeminiKey:       os.Getenv("GEMINI_API_KEY"),
		Model:           env("GEMINI_MODEL", "gemini-3.8-flash"),
		OpenRouterKey:   os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel: env("OPENROUTER_MODEL", "google/gemma-4-31b-it:free"),
		Port:            env("PORT", "8080"),
		TurnstileSecret: os.Getenv("TURNSTILE_SECRET_KEY"),
		Origins: map[string]bool{
			"http://localhost:5173": true,
			"http://localhost:8443": true,
		},
	}
	for _, o := range strings.Split(os.Getenv("FRONTEND_URL"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.Origins[o] = true
		}
	}
	return c
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" || os.Getenv(k) != "" {
			continue
		}
		if uq, err := strconv.Unquote(v); err == nil {
			v = uq
		} else {
			v = strings.Trim(v, "\"'")
		}
		_ = os.Setenv(k, v)
	}
}
