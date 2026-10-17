# Nutrily

Monorepo lokal berisi dua proyek terpisah: frontend React di `nutrily/` dan backend Go di `backend/`. Masing-masing punya Git repository sendiri.

## Backend

1. Dari `backend/`, salin `.env.example` menjadi `.env`; isi kredensial Supabase, key Gemini di `GEMINI_API_KEYS` untuk analisis gambar chat, key Groq di `GROQ_API_KEYS` untuk saran gizi dan chat, dan key OpenAI di `OPENAI_API_KEY` untuk OCR scan serta pencarian web saat estimasi gizi tak berkatalog. Pisahkan key multi dengan koma.
2. Jalankan `schema.sql` di Supabase SQL Editor. Untuk database lama, jalankan `migrations/011_meal_nutrition_sources.sql` agar sumber gizi tersimpan di riwayat.
3. Jalankan `go run ./cmd/api`.

Frontend mengirim Supabase access token. API memvalidasi token via Supabase Auth sebelum mengakses data pengguna; Supabase RLS membatasi akses tabel. Backend memproses gambar dengan OpenAI untuk OCR, mencocokkan hasil ke katalog CSV lalu cache produk (barcode+variant), dan memakai pencarian web OpenAI untuk estimasi gizi tak berkatalog; lalu menggunakan Groq untuk saran gizi dan chat, dengan Gemini untuk analisis gambar chat.

Model default: `gpt-4o-mini` untuk OCR dan pencarian web; `gemini-3.8-flash` untuk analisis gambar chat; `openai/gpt-oss-20b` untuk saran gizi dan jawaban chat. GPT-OSS 20B model production dan memiliki batas Free Plan.

Endpoint: `/health`, `/api/profile`, `/api/dashboard`, `/api/meals`, `/api/scan`, `/api/chat`.

## Frontend

1. Dari `nutrily/`, salin `.env.example` menjadi `.env.local`.
2. Isi `VITE_SUPABASE_URL`, `VITE_SUPABASE_ANON_KEY`, dan `VITE_API_URL`.
3. Jalankan `npm install` lalu `npm run dev`.

Aktifkan Google provider di Supabase Auth, isi Google OAuth client ID/secret, lalu tambahkan URL callback Supabase yang ditampilkan di Auth Providers ke Google Cloud Console. Tambahkan URL app lokal/deploy ke Supabase Auth URL Configuration.
