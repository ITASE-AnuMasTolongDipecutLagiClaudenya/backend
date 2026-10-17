package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SupabaseURL       string
	AnonKey           string
	GeminiKeys        []string
	GeminiModel       string
	GroqKeys          []string
	GroqModel         string
	OpenAIKey         string
	OpenAIModel       string
	ExaKey            string
	ExaNumResults     int
	ExaTrustedDomains []string
	CloudinaryCloud   string
	CloudinaryKey     string
	CloudinarySecret  string
	Port              string
	TurnstileSecret   string
	Origins           map[string]bool

	// Spam/budget controls for the OCR + search pipeline.
	SearchMaxResults   int
	ScanDailyQuota     int
	ScanMonthlyQuota   int
	ProductCacheMaxAge time.Duration
}

func Load() Config {
	loadEnv()
	c := Config{
		SupabaseURL:        strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		AnonKey:            os.Getenv("SUPABASE_ANON_KEY"),
		GeminiKeys:         keys("GEMINI_API_KEYS", "GEMINI_API_KEY"),
		GeminiModel:        env("GEMINI_MODEL", "gemini-3.8-flash"),
		GroqKeys:           keys("GROQ_API_KEYS", "GROQ_API_KEY"),
		GroqModel:          env("GROQ_MODEL", "openai/gpt-oss-20b"),
		OpenAIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:        env("OPENAI_MODEL", "gpt-4o-mini"),
		ExaKey:             os.Getenv("EXA_API_KEY"),
		ExaNumResults:      envInt("EXA_NUM_RESULTS", 3),
		ExaTrustedDomains:  keys("EXA_TRUSTED_DOMAINS", ""),
		CloudinaryCloud:    strings.TrimSpace(os.Getenv("CLOUDINARY_CLOUD_NAME")),
		CloudinaryKey:      strings.TrimSpace(os.Getenv("CLOUDINARY_API_KEY")),
		CloudinarySecret:   strings.TrimSpace(os.Getenv("CLOUDINARY_API_SECRET")),
		Port:               env("PORT", "8080"),
		TurnstileSecret:    os.Getenv("TURNSTILE_SECRET_KEY"),
		SearchMaxResults:   envInt("SCAN_SEARCH_MAX_RESULTS", 5),
		ScanDailyQuota:     envInt("SCAN_DAILY_QUOTA", 200),
		ScanMonthlyQuota:   envInt("SCAN_MONTHLY_QUOTA", 5000),
		ProductCacheMaxAge: envDuration("PRODUCT_CACHE_MAX_AGE", 30*24*time.Hour),
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

func keys(primary, fallback string) []string {
	value := os.Getenv(primary)
	if value == "" {
		value = os.Getenv(fallback)
	}
	var result []string
	for _, key := range strings.Split(value, ",") {
		if key = strings.TrimSpace(key); key != "" {
			result = append(result, key)
		}
	}
	return result
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil && d > 0 {
			return d
		}
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
