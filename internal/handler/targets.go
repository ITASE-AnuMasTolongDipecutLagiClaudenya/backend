package handler

import (
	"math"
	"net/http"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/nutrition"
)

func (h *Handler) CalculateTargets(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	if body == nil {
		httpx.WriteError(w, 400, "Body profil wajib berupa objek JSON")
		return
	}
	for key := range body {
		if !oneOf(key, "age", "gender", "pregnancy_status", "weight_kg", "height_cm", "activity", "goal", "is_gym") {
			httpx.WriteError(w, 400, "Field profil tidak dikenal: "+key)
			return
		}
	}
	age, ageOK := body["age"].(float64)
	weight, weightOK := body["weight_kg"].(float64)
	height, heightOK := body["height_cm"].(float64)
	gender, genderOK := body["gender"].(string)
	activity, activityOK := body["activity"].(string)
	goal, goalOK := body["goal"].(string)
	if !ageOK || age < 13 || age > 120 || age != math.Trunc(age) ||
		!weightOK || weight < 20 || weight > 200 ||
		!heightOK || height < 30 || height > 200 {
		httpx.WriteError(w, 400, "Usia, berat, atau tinggi tidak valid")
		return
	}
	if !genderOK || !oneOf(gender, "Pria", "Wanita") ||
		!activityOK || !oneOf(activity, "Minim gerak", "Ringan", "Sedang", "Sangat aktif", "Aktivitas fisik berat") ||
		!goalOK || !oneOf(goal, "Menurunkan berat badan", "Menjaga berat badan", "Menambah berat badan") {
		httpx.WriteError(w, 400, "Profil belum lengkap atau tidak valid")
		return
	}
	profile := map[string]any{
		"age": age, "gender": gender, "weight_kg": weight, "height_cm": height,
		"activity": activity, "goal": goal,
	}
	gymValue, gymOK := body["is_gym"].(bool)
	if !gymOK {
		httpx.WriteError(w, 400, "Status gym harus berupa ya atau tidak")
		return
	}
	profile["is_gym"] = gymValue
	if status, exists := body["pregnancy_status"]; exists && status != nil {
		statusValue, ok := status.(string)
		if !ok || !oneOf(statusValue, "Tidak ada", "Trimester 1", "Trimester 2", "Trimester 3", "Menyusui 6 bulan pertama", "Menyusui 6 bulan kedua") {
			httpx.WriteError(w, 400, "Status kehamilan tidak valid")
			return
		}
		profile["pregnancy_status"] = statusValue
	}
	if gender == "Wanita" {
		if status, ok := profile["pregnancy_status"].(string); !ok || status == "" {
			httpx.WriteError(w, 400, "Status kehamilan wajib diisi")
			return
		}
	}
	addition := false
	if gender == "Wanita" {
		status, _ := profile["pregnancy_status"].(string)
		addition = oneOf(status, "Trimester 1", "Trimester 2", "Trimester 3", "Menyusui 6 bulan pertama", "Menyusui 6 bulan kedua")
	}
	httpx.WriteJSON(w, 200, map[string]any{
		"targets":  nutrition.Targets(profile),
		"addition": addition,
	})
}
