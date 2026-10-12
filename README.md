# Nutrily Backend

API Go untuk Nutrily; frontend React terpisah di `../nutrily`.

## Menjalankan

1. Salin `.env.example` menjadi `.env`; isi URL dan anon key Supabase serta `GEMINI_API_KEY` dari Google AI Studio. Set `TURNSTILE_SECRET_KEY` ke secret Cloudflare Turnstile.
2. Jalankan `schema.sql` melalui Supabase SQL Editor. Untuk database lama, jalankan `migrations/002_feature_usage.sql` (serta `migrations/001_add_is_gym.sql` bila diperlukan).
3. Jalankan `go mod tidy`, kemudian `go run ./cmd/api`.

Backend memvalidasi bearer token terhadap Supabase Auth, lalu meneruskan token itu ke Supabase REST. RLS memastikan user hanya membaca/menulis datanya sendiri; service-role key tidak diperlukan.

Endpoint: `GET /health`, public `POST /api/auth/verify-turnstile`, `GET|PUT /api/profile`, `POST /api/targets`, `GET /api/dashboard`, `GET /api/calories`, `GET /api/features`, `GET|POST /api/meals`, `POST /api/scan`, `GET|POST /api/chat`.

Frontend config: set `VITE_TURNSTILE_SITE_KEY` in `../nutrily/.env`. For local testing, Cloudflare test keys are `1x00000000000000000000AA` (site) and `1x0000000000000000000000000000000AA` (secret); replace with matching production keys before deployment. Keep Supabase CAPTCHA disabled: the Go verification endpoint consumes this single-use token before OAuth.

Check: `go test ./...`
