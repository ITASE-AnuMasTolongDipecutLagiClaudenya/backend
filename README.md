# Nutrily Backend

API Go untuk Nutrily; frontend React terpisah di `../nutrily`.

## Menjalankan

1. Salin `.env.example` menjadi `.env`; isi URL dan anon key Supabase serta `GEMINI_API_KEY` dari Google AI Studio. Default `GEMINI_MODEL=gemini-3.8-flash`, model Flash terbaru dengan Free Tier tercantum di harga resmi Google.
2. Jalankan `schema.sql` melalui Supabase SQL Editor.
3. Jalankan `go mod tidy`, kemudian `go run ./cmd/api`.

Backend memvalidasi bearer token terhadap Supabase Auth, lalu meneruskan token itu ke Supabase REST. RLS memastikan user hanya membaca/menulis datanya sendiri; service-role key tidak diperlukan.

Endpoint: `GET /health`, `GET|PUT /api/profile`, `GET /api/dashboard`, `GET|POST /api/meals`, `POST /api/scan`, `GET|POST /api/chat`.

Check: `go test ./...`
