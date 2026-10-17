package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SupabaseURL      string
	AnonKey          string
	GeminiKeys       []string
	GeminiModel      string
	GroqKeys         []string
	GroqModel        string
	CloudinaryCloud  string
	CloudinaryKey    string
	CloudinarySecret string
	Port             string
	TurnstileSecret  string
	Origins          map[string]bool
}

func Load() Config {
	loadEnv()
	c := Config{
		SupabaseURL:      strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		AnonKey:          os.Getenv("SUPABASE_ANON_KEY"),
		GeminiKeys:       keys("GEMINI_API_KEYS", "GEMINI_API_KEY"),
		GeminiModel:      env("GEMINI_MODEL", "gemini-3.8-flash"),
		GroqKeys:         keys("GROQ_API_KEYS", "GROQ_API_KEY"),
		GroqModel:        env("GROQ_MODEL", "openai/gpt-oss-20b"),
		CloudinaryCloud:  strings.TrimSpace(os.Getenv("CLOUDINARY_CLOUD_NAME")),
		CloudinaryKey:    strings.TrimSpace(os.Getenv("CLOUDINARY_API_KEY")),
		CloudinarySecret: strings.TrimSpace(os.Getenv("CLOUDINARY_API_SECRET")),
		Port:             env("PORT", "8080"),
		TurnstileSecret:  os.Getenv("TURNSTILE_SECRET_KEY"),
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
