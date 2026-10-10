package nutrition

import "math"

func Targets(profile map[string]any) map[string]float64 {
	weight := ToFloat(profile["weight_kg"])
	height := ToFloat(profile["height_cm"])
	age := ToFloat(profile["age"])
	if weight <= 0 || height <= 0 || age <= 0 {
		return map[string]float64{"calories": 2100, "protein_g": 100, "carbs_g": 260, "fat_g": 70}
	}
	sexOffset := -161.0
	if profile["gender"] == "Pria" {
		sexOffset = 5
	}
	bmr := 10*weight + 6.25*height - 5*age + sexOffset
	activity := 1.375
	switch profile["activity"] {
	case "Minim gerak":
		activity = 1.2
	case "Sedang":
		activity = 1.55
	case "Sangat aktif":
		activity = 1.725
	case "Aktivitas fisik berat":
		activity = 1.9
	}
	calories := bmr * activity
	switch profile["goal"] {
	case "Menurunkan berat badan":
		calories -= 300
	case "Menambah berat badan":
		calories += 250
	}
	calories = math.Max(1200, math.Min(5000, calories))
	protein := weight * 1.6
	fat := calories * .25 / 9
	return map[string]float64{"calories": math.Round(calories), "protein_g": math.Round(protein), "fat_g": math.Round(fat), "carbs_g": math.Round((calories - protein*4 - fat*9) / 4)}
}
