package handler

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/nutrition"
)

const labelPrompt = "Baca label gizi pada gambar. Balas HANYA JSON valid tanpa markdown: {\"name\":..., \"serving_size\":..., \"calories\":..., \"protein_g\":..., \"carbs_g\":..., \"fat_g\":..., \"sugar_g\":..., \"sodium_mg\":..., \"note\":...}. Gunakan angka; null untuk nilai tidak terbaca. Jangan mengarang."

const receiptPrompt = "Baca seluruh struk, termasuk bagian atas dan bawah. Periksa setiap baris barang satu per satu: OCR nama barang seperti tertulis, termasuk nama merek, singkatan, dan ejaan yang kurang jelas. Masukkan setiap baris yang mungkin merupakan makanan, minuman, atau buah untuk dikonsumsi, termasuk barang kemasan, snack, roti, mi, susu, kopi, teh, jus, air minum, dan buah. Jangan buang barang hanya karena nama/merek tidak dikenal atau teks struk disingkat; jika ragu apakah barang dapat dikonsumsi, tetap masukkan dan beri note \"Nama perlu diverifikasi\". Jangan gabungkan baris barang yang berbeda. Abaikan hanya baris yang jelas bukan barang konsumsi serta pajak, diskon, subtotal, total, kembalian, metode pembayaran, biaya layanan, dan kantong belanja. Jika jumlah N terbaca (misal 2x), kalikan semua angka gizi dengan N dan tulis serving_size seperti \"2x ...\"; jangan buat field jumlah terpisah. Estimasi gizi per sajian standar Indonesia; jangan menghilangkan barang karena nilai gizinya tidak diketahui, gunakan null untuk nilai yang tidak dapat diestimasi. Balas HANYA JSON valid tanpa markdown: {\"items\": [{\"name\":..., \"category\": \"makanan|minuman|buah\", \"serving_size\":..., \"calories\":..., \"protein_g\":..., \"carbs_g\":..., \"fat_g\":..., \"sugar_g\":..., \"sodium_mg\":..., \"note\":...}]}. Satu objek untuk setiap baris barang konsumsi. Jika tidak ada, balas {\"items\": []}. Maksimal 50 barang."

func (h *Handler) Scan(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if h.AI.APIKey == "" {
		httpx.WriteError(w, 503, "Pemindaian memerlukan GEMINI_API_KEY di backend")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err := r.ParseMultipartForm(9 << 20); err != nil {
		httpx.WriteError(w, 400, "Gambar tidak valid atau melebihi 8 MB")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		httpx.WriteError(w, 400, "File gambar wajib disertakan")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		httpx.WriteError(w, 400, "Gambar melebihi 8 MB")
		return
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		httpx.WriteError(w, 400, "Konten file bukan gambar yang valid")
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		httpx.WriteError(w, 400, "Gunakan gambar JPG atau PNG")
		return
	}
	mode := strings.TrimSpace(r.FormValue("mode"))
	if mode == "" {
		mode = "label"
	}
	if mode != "label" && mode != "receipt" {
		httpx.WriteError(w, 400, "Mode pindai tidak valid")
		return
	}
	prompt := labelPrompt
	if mode == "receipt" {
		prompt = receiptPrompt
	}
	contents := []any{map[string]any{"role": "user", "parts": []any{
		map[string]string{"text": prompt},
		map[string]any{"inlineData": map[string]string{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(data)}},
	}}}
	result, err := h.AI.Generate(r.Context(), "", contents, true)
	if err != nil {
		httpx.WriteError(w, 502, "Pemindaian gagal: "+err.Error())
		return
	}
	h.recordFeatureUsage(r, "scan_"+mode)
	httpx.WriteJSON(w, 200, map[string]any{
		"mode":    mode,
		"items":   nutrition.NormalizeScanItems(result),
		"user_id": u.ID,
	})
}
