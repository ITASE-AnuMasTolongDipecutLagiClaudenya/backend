# Nutrily Backend

API Go untuk Nutrily; frontend React terpisah di `../nutrily`.

## Menjalankan

1. Salin `.env.example` menjadi `.env`; isi URL dan anon key Supabase, enam key Gemini (`GEMINI_API_KEYS`) untuk OCR, estimasi gizi dengan Google Search grounding, serta analisis gambar chat; dan enam key Groq (`GROQ_API_KEYS`) untuk saran gizi dan jawaban chat. Model default `openai/gpt-oss-20b` berstatus production, bukan Preview, dan tercantum pada batas rate Free Plan. Free Plan tunduk pada kuota; Developer Plan menagih $0.075/1M input tokens dan $0.30/1M output tokens. Output dibatasi 768 token. Key dipisahkan koma dan dirotasi tiap request. Isi `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, dan `CLOUDINARY_API_SECRET` dari Cloudinary untuk upload avatar. Set `TURNSTILE_SECRET_KEY` ke secret Cloudflare Turnstile.
2. Jalankan `schema.sql` melalui Supabase SQL Editor. Untuk database lama, jalankan migrations yang diperlukan, termasuk `011_meal_nutrition_sources.sql` agar sumber gizi tersimpan di riwayat pemindaian. Jalankan migrations `003_chat_delete.sql`, `004_chat_session_id.sql`, `005_chat_delete_policy.sql`, dan `007_meal_scan_session.sql` pada project Supabase yang dipakai aplikasi. Jalankan `001_add_is_gym.sql` bila diperlukan.
3. Jalankan `go mod tidy`, kemudian `go run ./cmd/api`.

Backend memvalidasi bearer token terhadap Supabase Auth, lalu meneruskan token itu ke Supabase REST. RLS memastikan user hanya membaca/menulis datanya sendiri; service-role key tidak diperlukan.

Endpoint: `GET /health`, public `POST /api/auth/verify-turnstile`, `GET|PUT /api/profile`, `POST|DELETE /api/profile/avatar`, `POST /api/targets`, `GET /api/dashboard`, `GET /api/calories`, `GET /api/features`, `GET|POST /api/meals`, `POST /api/scan`, `GET|POST /api/chat`, `GET|DELETE /api/conversations`.

Frontend config: set `VITE_TURNSTILE_SITE_KEY` in `../nutrily/.env`. For local testing, Cloudflare test keys are `1x00000000000000000000AA` (site) and `1x0000000000000000000000000000000AA` (secret); replace with matching production keys before deployment. Keep Supabase CAPTCHA disabled: the Go verification endpoint consumes this single-use token before OAuth.

Check: `go test ./...`
