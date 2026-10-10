package handler

import (
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/supabase"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	profile, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "profiles", supabase.UserFilter(u.ID)+"&select=*")
	if err != nil {
		log.Printf("dashboard profile query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca profil")
		return
	}
	today := time.Now().In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02")
	meals, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "meals", supabase.UserFilter(u.ID)+"&created_at=gte."+url.QueryEscape(today+"T00:00:00+07:00")+"&order=created_at.desc&limit=100")
	if err != nil {
		log.Printf("dashboard meals query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca catatan makan")
		return
	}
	totals := map[string]float64{"calories": 0, "protein_g": 0, "carbs_g": 0, "fat_g": 0}
	for _, meal := range meals {
		for _, k := range []string{"calories", "protein_g", "carbs_g", "fat_g"} {
			if n, ok := meal[k].(float64); ok {
				totals[k] += n
			}
		}
	}
	httpx.WriteJSON(w, 200, map[string]any{"profile": nutrition.First(profile), "meals": meals, "totals": totals, "targets": nutrition.Targets(nutrition.First(profile))})
}

func (h *Handler) ListMeals(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "meals", supabase.UserFilter(u.ID)+"&order=created_at.desc&limit=100")
	if err != nil {
		log.Printf("meal list query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca catatan makan")
		return
	}
	httpx.WriteJSON(w, 200, rows)
}

func (h *Handler) CreateMeal(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	var body map[string]any
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	if body == nil {
		httpx.WriteError(w, 400, "Body makanan wajib berupa objek JSON")
		return
	}
	name, ok := body["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		httpx.WriteError(w, 400, "Nama makanan wajib diisi")
		return
	}
	if len(name) > 160 {
		httpx.WriteError(w, 400, "Nama makanan terlalu panjang")
		return
	}
	allowed := map[string]bool{"user_id": true, "name": true, "serving_size": true, "calories": true, "protein_g": true, "carbs_g": true, "fat_g": true, "sugar_g": true, "sodium_mg": true}
	for k := range body {
		if !allowed[k] {
			httpx.WriteError(w, 400, "Field makanan tidak dikenal: "+k)
			return
		}
	}
	for _, f := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
		if raw, exists := body[f]; exists {
			v, ok := raw.(float64)
			if !ok || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				httpx.WriteError(w, 400, "Nilai "+f+" harus berupa angka positif")
				return
			}
		}
	}
	body["user_id"] = u.ID
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "POST", "meals", body)
	if err != nil || len(rows) == 0 {
		if err != nil {
			log.Printf("meal save: %v", err)
		}
		httpx.WriteError(w, 502, "Gagal menyimpan makanan; periksa skema Supabase")
		return
	}
	httpx.WriteJSON(w, 201, rows[0])
}
