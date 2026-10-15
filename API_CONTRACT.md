# Nutrily API Contract

Kontrak mengikuti implementasi backend Go saat ini, bukan daftar endpoint lama di README.

## Dasar

- **Base URL:** nilai `VITE_API_URL`; default `http://localhost:8080`.
- **Format:** JSON UTF-8, kecuali upload multipart.
- **Autentikasi:** semua `/api/*` selain `/api/auth/verify-turnstile` memerlukan `Authorization: Bearer <Supabase access token>`. Backend memvalidasi token lewat Supabase Auth; akses data memakai token dan RLS.
- **Body JSON:** maksimal 1 MiB dan tepat satu nilai JSON.
- **Error:** `{"error":"pesan"}`.
- **CORS:** origin lokal `http://localhost:5173` dan `http://localhost:8443`, ditambah origin dari `FRONTEND_URL`. `OPTIONS` pada route API mengembalikan `204`.
- Frontend API helper: `nutrily/src/lib/api.js`. Endpoint dipanggil oleh `nutrily/src/App.jsx`, `nutrily/src/components/ScannerFlow.jsx`, dan `nutrily/src/components/ui/calorie-chart.jsx`.

## Endpoint

| Path | Method | Dipakai frontend |
| --- | --- | --- |
| `/health` | `GET` | Tidak; health check backend |
| `/api/auth/verify-turnstile` | `POST` | Ya; login sebelum OAuth |
| `/api/profile` | `GET`, `PUT` | Ya; callback auth, onboarding, profil |
| `/api/targets` | `POST` | Ya; pratinjau onboarding |
| `/api/dashboard` | `GET` | Ya; dasbor |
| `/api/calories` | `GET` | Ya; grafik kalori |
| `/api/features` | `GET` | Ya; statistik fitur |
| `/api/meals` | `GET`, `POST` | Ya; riwayat, simpan hasil scan |
| `/api/scan` | `POST` | Ya; scan label/struk |
| `/api/chat` | `GET`, `POST` | Ya; riwayat dan chat AI |
| `/api/conversations` | `GET`, `DELETE` | Ya; daftar/hapus percakapan |

Total: 11 path, 15 kombinasi method-path termasuk `GET /health` (14 endpoint API; seluruh 14 endpoint API dipanggil frontend).

## Integrasi eksternal

Endpoint berikut dipanggil komponen aplikasi, bukan endpoint publik backend Nutrily:

| Host/path | Pemanggil | Tujuan |
| --- | --- | --- |
| `{SUPABASE_URL}/auth/v1/user` | Backend | Validasi access token |
| `{SUPABASE_URL}/rest/v1/{profiles,meals,chat_messages,rpc/*}` | Backend | Baca/tulis data melalui Supabase REST dan RLS |
| `https://challenges.cloudflare.com/turnstile/v0/siteverify` | Backend | Verifikasi token Turnstile |
| `https://generativelanguage.googleapis.com/v1beta/models/{GEMINI_MODEL}:generateContent` | Backend | OCR/chat; utama bila Gemini dikonfigurasi |
| `https://openrouter.ai/api/v1/chat/completions` | Backend | Fallback AI bila Gemini gagal/tidak tersedia |
| Supabase Auth OAuth/user endpoints | Frontend Supabase SDK | Login Google, session, pembaruan kata sandi |

### `GET /health`

Pemeriksaan proses backend; tanpa autentikasi. Bukan pemanggilan dari frontend.

```json
{"status":"ok"}
```

### `POST /api/auth/verify-turnstile`

Public; dipakai alur login sebelum OAuth. Body JSON:

```json
{"token":"<Cloudflare Turnstile token>"}
```

Token wajib, maksimum 2048 karakter. Respons `200`: `{"ok":true}`. Error khusus: `400` token/body tidak valid, `403` verifikasi ditolak, `503` secret belum dikonfigurasi, `502` layanan Turnstile gagal.

### `GET /api/profile`

Profil pengguna aktif. Respons `200`: satu objek profil, atau `{}` bila belum ada.

### `PUT /api/profile`

Simpan/upsert profil pengguna aktif. Field `user_id` dari klien diabaikan dan ditetapkan dari token. Field yang diterima: `full_name`, `age`, `gender`, `pregnancy_status`, `weight_kg`, `height_cm`, `activity`, `goal`, `is_gym`.

```json
{
  "full_name":"Ayu",
  "age":28,
  "gender":"Wanita",
  "pregnancy_status":"Tidak ada",
  "weight_kg":60,
  "height_cm":160,
  "activity":"Sedang",
  "goal":"Menjaga berat badan",
  "is_gym":false
}
```

Validasi: usia 13–120, berat 1–500 kg, tinggi 30–275 cm; `gender`: `Pria` atau `Wanita`; `activity`: `Minim gerak`, `Ringan`, `Sedang`, `Sangat aktif`, `Aktivitas fisik berat`; `goal`: `Menurunkan berat badan`, `Menjaga berat badan`, `Menambah berat badan`; `is_gym` boolean. `pregnancy_status` bila berupa string harus salah satu dari `Tidak ada`, `Trimester 1`, `Trimester 2`, `Trimester 3`, `Menyusui 6 bulan pertama`, `Menyusui 6 bulan kedua`; `null` diterima. `full_name` wajib berupa string (boleh kosong), maksimum 120 byte setelah trim. Field asing ditolak. Respons `200`: objek profil tersimpan beserta `target_calories`, `target_protein_g`, `target_carbs_g`, `target_fat_g`, `target_sugar_g`, `target_sodium_mg`, dan `updated_at`.

### `POST /api/targets`

Hitung pratinjau target tanpa menyimpan. Body: `age`, `gender`, `weight_kg`, `height_cm`, `activity`, `goal`, `is_gym`; `pregnancy_status` opsional untuk `Pria`, wajib untuk `Wanita`. Nilai dan enum sama seperti `PUT /api/profile`, kecuali `age` harus bilangan bulat. Field asing ditolak.

```json
{
  "age":28,
  "gender":"Wanita",
  "pregnancy_status":"Tidak ada",
  "weight_kg":60,
  "height_cm":160,
  "activity":"Sedang",
  "goal":"Menjaga berat badan",
  "is_gym":false
}
```

Respons `200`: `{"targets":{"calories":0,"protein_g":0,"carbs_g":0,"fat_g":0,"sugar_g":0,"sodium_mg":0},"addition":false}`. Angka contoh `0` mewakili nilai hasil hitung.

### `GET /api/dashboard`

Data dasbor hari ini menurut WIB; memuat maksimum 100 catatan makan terbaru hari ini.

```json
{
  "profile": {},
  "meals": [],
  "totals": {"calories":0,"protein_g":0,"carbs_g":0,"fat_g":0,"sugar_g":0,"sodium_mg":0},
  "targets": {"calories":0,"protein_g":0,"carbs_g":0,"fat_g":0,"sugar_g":0,"sodium_mg":0}
}
```

`profile` kosong bila belum ada profil. `meals` memakai bentuk catatan makan pada endpoint meals.

### `GET /api/calories?period=<period>`

Riwayat gizi untuk grafik. `period` wajib: `day`, `week`, `month`, atau `year`. Setiap bucket memuat `calories`, `protein_g`, `carbs_g`, `fat_g`, `sugar_g`, dan `sodium_mg`; nilai nutrisi yang tidak tersedia dijumlahkan sebagai `0`. Implementasi saat ini: `day` mengembalikan tujuh bucket hari pada minggu berjalan; `week` empat bucket pada bulan berjalan; `month` dua belas bulan pada tahun berjalan; `year` bucket tetap 2027–2032.

### `GET /api/features`

Jumlah penggunaan fitur pengguna aktif. Respons `200`:

```json
{"label":0,"receipt":0,"chatbot":0}
```

### `GET /api/meals`

Daftar maksimum 1000 catatan makan terbaru pengguna aktif. Respons `200`: array objek meal (`id`, `user_id`, `name`, `serving_size`, nilai gizi, `scan_session_id`, `scan_session_name`, `scan_mode`, `created_at`). `scan_session_id` opsional; jika diberikan, hanya catatan sesi itu dikembalikan (urut lama ke baru).

### `POST /api/meals`

Simpan catatan makan. `name` wajib (maksimum 160 byte, tidak boleh kosong setelah trim). Field opsional: `serving_size`, nilai gizi, `scan_session_id` (UUID), `scan_session_name` (maksimum 60 karakter, tidak boleh kosong), `scan_mode` (`label` atau `receipt`). Nilai gizi harus angka finite ≥ 0. `user_id` boleh tidak dikirim; bila dikirim akan ditimpa dengan ID pengguna terautentikasi. Field asing ditolak.


```json
{
  "name":"Nasi ayam",
  "serving_size":"1 porsi",
  "calories":520,
  "protein_g":28,
  "carbs_g":62,
  "fat_g":17,
  "sugar_g":4,
  "sodium_mg":640
}
```

Respons `201`: objek meal tersimpan.
### `POST /api/profile/avatar`

Upload avatar multipart field `avatar` (JPG/PNG, maksimum 5 MB). Backend memotong area tengah menjadi persegi dan menyimpan gambar ke Cloudinary. Respons `200`: `{ "avatar_url": "...", "public_id": "..." }`.

### `DELETE /api/profile/avatar`

Hapus avatar Cloudinary pengguna aktif. Respons sukses `204` tanpa body.

### `PATCH /api/meals?scan_session_id=<UUID>` atau `?meal_id=<ID>`

Ubah nama sesi (`scan_session_name`) untuk semua catatan dalam sesi, atau satu catatan lama tanpa ID sesi. Body: `{"scan_session_name":"Scan label 1"}`.

### `DELETE /api/meals?scan_session_id=<UUID>` atau `?meal_id=<ID>`

Hapus semua catatan dalam sesi atau satu catatan lama tanpa ID sesi.

### `POST /api/scan`

Scan label gizi atau struk. `multipart/form-data`: `image` wajib, JPG/PNG maksimum 8 MiB; `mode` opsional (`label` default atau `receipt`). Backend mencocokkan nama/barcode ke `data/gizi_lookup.csv` hanya pada baris `bisa_disajikan=1`, `basis_gizi=per_100g`, dan kualitas terpercaya. Hasil membawa `nutrition_source`: `catalog` untuk nilai CSV yang diskalakan ke porsi, `label` untuk angka yang terbaca dari label, atau `estimate` untuk fallback AI. Nilai yang tidak diketahui tetap `null`, bukan `0`. Respons `200`:

```json
{
  "mode":"label",
  "items":[{
    "name":"Susu",
    "serving_size":"250 ml",
    "calories":120,
    "protein_g":8,
    "carbs_g":12,
    "fat_g":4,
    "sugar_g":10,
    "sodium_mg":100,
    "nutrition_source":"catalog",
    "note":""
  }],
  "user_id":"<UUID>"
}
```

`category` hanya dikembalikan jika dikenali. Mode struk memfilter kategori ke `makanan`, `minuman`, `buah`. Scan tidak menyimpan meal otomatis. Error khusus: `400` gambar/mode tidak valid, `503` AI tidak dikonfigurasi, `502` pemindaian gagal.

### `GET /api/chat?conversation_id=<UUID>`

Ambil riwayat chat (maksimum 100 pesan, urut lama ke baru). `conversation_id` wajib UUID. Bentuk pesan: `id`, `role` (`user` atau `assistant`), `content`, `created_at`.

### `POST /api/chat`

Kirim pesan ke AI dan simpan pasangan pesan user/assistant. Dua format:

- `application/json`: `{"conversation_id":"<UUID>","message":"Apa kandungan gizinya?"}`
- `multipart/form-data`: `conversation_id` wajib, `message` opsional bila ada `image`; `image` opsional JPG/PNG maksimum 8 MiB.

ID percakapan wajib UUID. Pesan maksimum 4000 karakter; teks kosong hanya diterima bersama gambar. Respons `200`: `{"answer":"..."}`. Gambar tidak disimpan sebagai file; riwayat menandainya sebagai `[Gambar terlampir]`. Error khusus: `400` input/UUID/gambar tidak valid, `503` AI tidak dikonfigurasi, `502` konteks, AI, atau penyimpanan gagal.

### `GET /api/conversations`

Daftar percakapan terbaru pengguna aktif (dibentuk dari maksimum 500 pesan terakhir, satu entri per UUID). Respons `200`:

```json
[{"id":"<UUID>","preview":"Pesan terakhir","updated_at":"2026-01-01T00:00:00Z"}]
```

### `DELETE /api/conversations?conversation_id=<UUID>`

Hapus seluruh pesan percakapan milik pengguna aktif. `conversation_id` wajib UUID. Respons sukses `204` tanpa body; bila tidak ada baris terhapus, respons `404`. Pastikan policy dan grant DELETE pada Supabase aktif.

## Status umum

- `400`: body, field, parameter, atau upload tidak valid.
- `401`: bearer token tidak ada/tidak valid/kedaluwarsa.
- `403`: verifikasi Turnstile ditolak.
- `404`: endpoint tidak ditemukan (method salah pada route API juga jatuh ke respons ini).
- `405`: method salah pada `/api/auth/verify-turnstile`.
- `502`: Supabase, Turnstile, AI, atau operasi backend gagal.
- `503`: layanan AI/Turnstile belum dikonfigurasi.

Route terdaftar: `backend/internal/server/server.go`, dispatch endpoint API: `backend/internal/handler/api.go`.
