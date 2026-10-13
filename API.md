# Nutrily

Monorepo lokal berisi dua proyek terpisah: frontend React di `nutrily/` dan backend Go di `backend/`. Masing-masing punya Git repository sendiri.

## Backend

1. Dari `backend/`, salin `.env.example` menjadi `.env`; isi kredensial Supabase dan `GEMINI_API_KEY` dari Google AI Studio.
2. Jalankan `schema.sql` di Supabase SQL Editor.
3. Jalankan `go run ./cmd/api`.

Frontend mengirim Supabase access token. API memvalidasi token via Supabase Auth sebelum mengakses data pengguna; Supabase RLS membatasi akses tabel. Backend memanggil Gemini untuk OCR dan chat.

Default model `gemini-3.8-flash` (Free Tier tercantum di harga resmi Google; kuota dan ketersediaan dapat berubah).

Endpoint: `/health`, `/api/profile`, `/api/dashboard`, `/api/meals`, `/api/scan`, `/api/chat`.

## Frontend

1. Dari `nutrily/`, salin `.env.example` menjadi `.env.local`.
2. Isi `VITE_SUPABASE_URL`, `VITE_SUPABASE_ANON_KEY`, dan `VITE_API_URL`.
3. Jalankan `npm install` lalu `npm run dev`.

Aktifkan Google provider di Supabase Auth, isi Google OAuth client ID/secret, lalu tambahkan URL callback Supabase yang ditampilkan di Auth Providers ke Google Cloud Console. Tambahkan URL app lokal/deploy ke Supabase Auth URL Configuration.
