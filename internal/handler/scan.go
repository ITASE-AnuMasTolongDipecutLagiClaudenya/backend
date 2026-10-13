package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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

const labelPrompt = `Identifikasi persis produk pada gambar label gizi. Baca nama produk, merek, barcode yang terlihat, dan takaran saji beserta gram/ml bila tertulis. Jangan menyalin atau menghitung angka gizi dari label. Jangan menebak identitas/barcode. Balas hanya JSON: {"items":[{"name":"...","manufacturer":"...","barcode":"...","serving_size":"...","portion_g":100,"portion_ml":null,"category":"makanan|minuman|buah"}]}. Gunakan null bila nilai tidak terbaca.`

const receiptPrompt = `Baca struk dan identifikasi setiap baris makanan/minuman/buah yang bisa dikonsumsi. Jangan gabungkan item; abaikan pajak, diskon, subtotal, total, pembayaran, layanan, dan barang non-makanan. Ambil nama, merek/varian jika tercantum, jumlah pembelian, dan porsi/berat hanya jika tercantum. Jangan menebak nilai gizi. Balas hanya JSON: {"items":[{"name":"...","manufacturer":"...","barcode":null,"quantity":1,"serving_size":"...","portion_g":null,"portion_ml":null,"category":"makanan|minuman|buah"}]}. Gunakan null bila data tidak terbaca; jangan menebak barcode atau berat.`

func estimatePrompt(items []map[string]any, mode string) string {
	encoded, _ := json.Marshal(items)
	if mode == "receipt" {
		return "Perkirakan nilai gizi per porsi untuk item struk yang tidak cocok dengan katalog. Jangan mengubah identitas/jumlah item. Jangan mengarang: nilai tidak diketahui harus null. Sertakan nutrition_source=estimate pada setiap item. JSON saja: {\"items\":[{\"name\":\"...\",\"serving_size\":\"...\",\"calories\":0,\"protein_g\":0,\"carbs_g\":0,\"fat_g\":0,\"sugar_g\":null,\"sodium_mg\":null,\"nutrition_source\":\"estimate\"}]}. Item: " + string(encoded)
	}
	return "Baca angka yang terlihat pada label produk untuk item yang tidak cocok dengan katalog. Label adalah sumber nilai, jangan estimasi angka yang tak terbaca; gunakan null. Jangan mengubah identitas produk. Sertakan nutrition_source=label. JSON saja: {\"items\":[{\"name\":\"...\",\"serving_size\":\"...\",\"calories\":0,\"protein_g\":0,\"carbs_g\":0,\"fat_g\":0,\"sugar_g\":null,\"sodium_mg\":null,\"nutrition_source\":\"label\"}]}. Item: " + string(encoded)
}
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if !h.HasAI() {
		httpx.WriteError(w, 503, "Pemindaian memerlukan GEMINI_API_KEY atau OPENROUTER_API_KEY di backend")
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
	identified, err := h.GenerateAI(r.Context(), "", contents, true)
	if err != nil {
		httpx.WriteError(w, 502, "Pemindaian gagal: "+err.Error())
		return
	}
	items := nutrition.NormalizeScanItems(identified, mode)
	matches, unmatched := nutrition.ApplyCatalog(h.Catalog, items)
	if len(unmatched) > 0 {
		fallbackContents := []any{map[string]any{"role": "user", "parts": []any{
			map[string]string{"text": estimatePrompt(unmatched, mode)},
			map[string]any{"inlineData": map[string]string{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(data)}},
		}}}
		fallbackResult, fallbackErr := h.GenerateAI(r.Context(), "", fallbackContents, true)
		if fallbackErr == nil {
			estimates := nutrition.NormalizeScanItems(fallbackResult, mode)
			for index := range estimates {
				estimates[index]["nutrition_source"] = "estimate"
			}
			matches = append(matches, estimates...)
		} else {
			for _, item := range unmatched {
				item["nutrition_source"] = "estimate"
				for _, key := range []string{"calories", "protein_g", "fat_g", "carbs_g", "sugar_g", "sodium_mg"} {
					item[key] = nil
				}
				matches = append(matches, item)
			}
		}
	}
	h.recordFeatureUsage(r, "scan_"+mode)
	httpx.WriteJSON(w, 200, map[string]any{"mode": mode, "items": matches, "user_id": u.ID})
}
