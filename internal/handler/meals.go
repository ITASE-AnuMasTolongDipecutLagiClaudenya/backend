package handler

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	totals := map[string]float64{"calories": 0, "protein_g": 0, "carbs_g": 0, "fat_g": 0, "sugar_g": 0, "sodium_mg": 0}
	for _, meal := range meals {
		for _, k := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			if n, ok := meal[k].(float64); ok {
				totals[k] += n
			}
		}
	}
	httpx.WriteJSON(w, 200, map[string]any{"profile": nutrition.First(profile), "meals": meals, "totals": totals, "targets": nutrition.Targets(nutrition.First(profile))})
}

type calorieBucket struct {
	Start time.Time
	End   time.Time
	Label string
}

type caloriePoint struct {
	Label    string  `json:"label"`
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
	SugarG   float64 `json:"sugar_g"`
	SodiumMG float64 `json:"sodium_mg"`
}

func calorieBuckets(period string, now time.Time) []calorieBucket {
	location := time.FixedZone("WIB", 7*60*60)
	now = now.In(location)
	months := [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	weekdays := [...]string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}
	var buckets []calorieBucket

	switch period {
	case "day":
		monday := time.Date(now.Year(), now.Month(), now.Day()-((int(now.Weekday())+6)%7), 0, 0, 0, 0, location)
		for i := range 7 {
			start := monday.AddDate(0, 0, i)
			month := months[int(start.Month())-1]
			buckets = append(buckets, calorieBucket{start, start.AddDate(0, 0, 1), fmt.Sprintf("%s, %02d %s", weekdays[i], start.Day(), month[:3])})
		}
	case "week":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		end := start.AddDate(0, 1, 0)
		for i := range 4 {
			bucketStart := start.AddDate(0, 0, i*7)
			bucketEnd := bucketStart.AddDate(0, 0, 7)
			if i == 3 || bucketEnd.After(end) {
				bucketEnd = end
			}
			buckets = append(buckets, calorieBucket{bucketStart, bucketEnd, fmt.Sprintf("Minggu %d", i+1)})
		}
	case "month":
		for month := range 12 {
			start := time.Date(now.Year(), time.Month(month+1), 1, 0, 0, 0, 0, location)
			buckets = append(buckets, calorieBucket{start, start.AddDate(0, 1, 0), months[month]})
		}
	case "year":
		for year := 2027; year <= 2032; year++ {
			start := time.Date(year, time.January, 1, 0, 0, 0, 0, location)
			buckets = append(buckets, calorieBucket{start, start.AddDate(1, 0, 0), fmt.Sprint(year)})
		}
	}
	return buckets
}

func aggregateCalories(rows []map[string]any, buckets []calorieBucket) []caloriePoint {
	points := make([]caloriePoint, len(buckets))
	for i, bucket := range buckets {
		points[i].Label = bucket.Label
	}
	for _, row := range rows {
		createdAt, ok := row["created_at"].(string)
		if !ok {
			continue
		}
		created, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			continue
		}
		created = created.In(buckets[0].Start.Location())
		for i, bucket := range buckets {
			if !created.Before(bucket.Start) && created.Before(bucket.End) {
				points[i].Calories += nutritionValue(row, "calories")
				points[i].ProteinG += nutritionValue(row, "protein_g")
				points[i].CarbsG += nutritionValue(row, "carbs_g")
				points[i].FatG += nutritionValue(row, "fat_g")
				points[i].SugarG += nutritionValue(row, "sugar_g")
				points[i].SodiumMG += nutritionValue(row, "sodium_mg")
				break
			}
		}
	}
	return points
}

func nutritionValue(row map[string]any, key string) float64 {
	value, _ := row[key].(float64)
	return value
}

func (h *Handler) CalorieHistory(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	buckets := calorieBuckets(r.URL.Query().Get("period"), time.Now())
	if len(buckets) == 0 {
		httpx.WriteError(w, 400, "Periode grafik kalori tidak valid")
		return
	}
	start, end := buckets[0].Start.UTC().Format(time.RFC3339), buckets[len(buckets)-1].End.UTC().Format(time.RFC3339)
	filters := fmt.Sprintf("%s&select=created_at,calories,protein_g,carbs_g,fat_g,sugar_g,sodium_mg&created_at=gte.%s&created_at=lt.%s&order=created_at.asc&limit=10000", supabase.UserFilter(u.ID), url.QueryEscape(start), url.QueryEscape(end))
	meals, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "meals", filters)
	if err != nil {
		log.Printf("calorie history query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca riwayat kalori")
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"data": aggregateCalories(meals, buckets)})
}

func (h *Handler) FeatureUsage(w http.ResponseWriter, r *http.Request, _ middleware.Identity) {
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "rpc/get_feature_usage")
	if err != nil {
		log.Printf("feature usage query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca statistik fitur; jalankan migrasi database terbaru")
		return
	}
	usage := map[string]int64{"label": 0, "receipt": 0, "chatbot": 0}
	for _, row := range rows {
		name, _ := row["feature"].(string)
		if count, ok := row["request_count"].(float64); ok {
			switch name {
			case "scan_label":
				usage["label"] = int64(count)
			case "scan_receipt":
				usage["receipt"] = int64(count)
			case "chatbot":
				usage["chatbot"] = int64(count)
			}
		}
	}
	httpx.WriteJSON(w, 200, usage)
}

func (h *Handler) recordFeatureUsage(r *http.Request, feature string) {
	_, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "POST", "rpc/record_feature_usage", map[string]string{"feature_name": feature})
	if err != nil {
		log.Printf("feature usage record (%s): %v", feature, err)
	}
}

func (h *Handler) ListMeals(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	filters := supabase.UserFilter(u.ID) + "&order=created_at.desc&limit=1000"
	if sessionID := strings.TrimSpace(r.URL.Query().Get("scan_session_id")); sessionID != "" {
		if !ValidUUID(sessionID) {
			httpx.WriteError(w, 400, "ID sesi pemindaian tidak valid")
			return
		}
		filters = supabase.UserFilter(u.ID) + "&scan_session_id=eq." + url.QueryEscape(sessionID) + "&order=created_at.asc&limit=100"
	}
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "meals", filters)
	if err != nil {
		log.Printf("meal list query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca catatan makan")
		return
	}
	httpx.WriteJSON(w, 200, rows)
}

func (h *Handler) UpdateMealSessionName(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	var body map[string]any
	if !httpx.DecodeBody(w, r, &body) || body == nil {
		return
	}
	value, ok := body["scan_session_name"].(string)
	if !ok {
		httpx.WriteError(w, 400, "Nama sesi pemindaian wajib diisi")
		return
	}
	sessionName := strings.TrimSpace(value)
	if sessionName == "" || utf8.RuneCountInString(sessionName) > 60 {
		httpx.WriteError(w, 400, "Nama sesi pemindaian wajib diisi (maksimum 60 karakter)")
		return
	}
	filter, ok := mealFilter(r)
	if !ok {
		httpx.WriteError(w, 400, "ID sesi pemindaian atau makanan tidak valid")
		return
	}
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), http.MethodPatch, "meals?"+supabase.UserFilter(u.ID)+"&"+filter, map[string]string{"scan_session_name": sessionName})
	if err != nil {
		log.Printf("meal session rename: %v", err)
		httpx.WriteError(w, 502, "Gagal mengubah nama sesi")
		return
	}
	if len(rows) == 0 {
		httpx.WriteError(w, 404, "Sesi pemindaian tidak ditemukan")
		return
	}
	httpx.WriteJSON(w, 200, rows)
}

func (h *Handler) DeleteMeals(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	filter, ok := mealFilter(r)
	if !ok {
		httpx.WriteError(w, 400, "ID sesi pemindaian atau makanan tidak valid")
		return
	}
	_, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), http.MethodDelete, "meals", supabase.UserFilter(u.ID)+"&"+filter)
	if err != nil {
		log.Printf("meal delete: %v", err)
		httpx.WriteError(w, 502, "Gagal menghapus catatan makanan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mealFilter(r *http.Request) (string, bool) {
	if sessionID := strings.TrimSpace(r.URL.Query().Get("scan_session_id")); sessionID != "" {
		if !ValidUUID(sessionID) {
			return "", false
		}
		return "scan_session_id=eq." + url.QueryEscape(sessionID), true
	}
	mealID, err := strconv.ParseInt(r.URL.Query().Get("meal_id"), 10, 64)
	if err != nil || mealID < 1 {
		return "", false
	}
	return "id=eq." + strconv.FormatInt(mealID, 10), true
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
	allowed := map[string]bool{"user_id": true, "name": true, "serving_size": true, "calories": true, "protein_g": true, "carbs_g": true, "fat_g": true, "sugar_g": true, "sodium_mg": true, "scan_session_id": true, "scan_session_name": true, "scan_mode": true}
	for k := range body {
		if !allowed[k] {
			httpx.WriteError(w, 400, "Field makanan tidak dikenal: "+k)
			return
		}
	}
	if id, exists := body["scan_session_id"]; exists {
		if value, ok := id.(string); !ok || !ValidUUID(value) {
			httpx.WriteError(w, 400, "ID sesi pemindaian tidak valid")
			return
		}
	}
	if mode, exists := body["scan_mode"]; exists {
		if value, ok := mode.(string); !ok || (value != "label" && value != "receipt") {
			httpx.WriteError(w, 400, "Mode pemindaian tidak valid")
			return
		}
	}
	if sessionName, exists := body["scan_session_name"]; exists {
		value, ok := sessionName.(string)
		if !ok || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 60 {
			httpx.WriteError(w, 400, "Nama sesi pemindaian wajib diisi (maksimum 60 karakter)")
			return
		}
		body["scan_session_name"] = strings.TrimSpace(value)
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
