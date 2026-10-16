package handler

import (
	"log"
	"net/http"
	"strings"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/supabase"
)

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "profiles", supabase.UserFilter(u.ID)+"&select=*")
	if err != nil {
		log.Printf("profile query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca profil")
		return
	}
	if len(rows) == 0 {
		httpx.WriteJSON(w, 200, map[string]any{})
		return
	}
	httpx.WriteJSON(w, 200, rows[0])
}

func (h *Handler) PutProfile(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	var body map[string]any
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	if body == nil {
		httpx.WriteError(w, 400, "Body profil wajib berupa objek JSON")
		return
	}
	for field, limits := range map[string][2]float64{"age": {13, 120}, "weight_kg": {20, 200}, "height_cm": {30, 200}} {
		value, ok := body[field].(float64)
		if !ok || value < limits[0] || value > limits[1] {
			httpx.WriteError(w, 400, "Nilai "+field+" tidak valid")
			return
		}
	}
	if gender, ok := body["gender"].(string); ok && gender != "Pria" && gender != "Wanita" {
		httpx.WriteError(w, 400, "Jenis kelamin tidak valid")
		return
	}
	if body["gender"] != "Pria" && body["gender"] != "Wanita" {
		httpx.WriteError(w, 400, "Jenis kelamin wajib diisi")
		return
	}
	if activity, ok := body["activity"].(string); !ok || !oneOf(activity, "Minim gerak", "Ringan", "Sedang", "Sangat aktif", "Aktivitas fisik berat") {
		httpx.WriteError(w, 400, "Aktivitas tidak valid")
		return
	}
	if _, ok := body["is_gym"].(bool); !ok {
		httpx.WriteError(w, 400, "Pilih apakah kamu rutin gym")
		return
	}
	if goal, ok := body["goal"].(string); !ok || !oneOf(goal, "Menurunkan berat badan", "Menjaga berat badan", "Menambah berat badan") {
		httpx.WriteError(w, 400, "Target tidak valid")
		return
	}
	name, ok := body["full_name"].(string)
	if !ok || strings.TrimSpace(name) == "" || len(strings.TrimSpace(name)) > 120 {
		httpx.WriteError(w, 400, "Nama profil wajib diisi dan maksimal 120 karakter")
		return
	}
	if gender := body["gender"]; gender == "Wanita" {
		status, ok := body["pregnancy_status"].(string)
		if !ok || strings.TrimSpace(status) == "" {
			httpx.WriteError(w, 400, "Status kehamilan atau menyusui wajib diisi")
			return
		}
	}
	if status, ok := body["pregnancy_status"].(string); ok && !oneOf(status, "Tidak ada", "Trimester 1", "Trimester 2", "Trimester 3", "Menyusui 6 bulan pertama", "Menyusui 6 bulan kedua") {
		httpx.WriteError(w, 400, "Status kehamilan tidak valid")
		return
	}
	allowed := map[string]bool{"user_id": true, "age": true, "gender": true, "pregnancy_status": true, "weight_kg": true, "height_cm": true, "activity": true, "goal": true, "full_name": true, "is_gym": true}
	for key := range body {
		if !allowed[key] {
			httpx.WriteError(w, 400, "Field profil tidak dikenal: "+key)
			return
		}
	}
	body["user_id"] = u.ID
	if name, ok := body["full_name"].(string); ok {
		body["full_name"] = strings.TrimSpace(name)
	} else {
		body["full_name"] = ""
	}
	for key, value := range nutrition.Targets(body) {
		body["target_"+key] = value
	}
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "POST", "profiles?on_conflict=user_id", body)
	if err != nil || len(rows) == 0 {
		if err != nil {
			log.Printf("profile save: %v", err)
		}
		httpx.WriteError(w, 502, "Gagal menyimpan profil; periksa skema Supabase")
		return
	}
	httpx.WriteJSON(w, 200, rows[0])
}

func oneOf(value string, choices ...string) bool {
	for _, c := range choices {
		if value == c {
			return true
		}
	}
	return false
}
