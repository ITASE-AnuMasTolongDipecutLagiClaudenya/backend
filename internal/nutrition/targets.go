package nutrition

import "math"

type akgBand struct {
	min, max     float64
	male, female [6]float64 // energy, protein, total fat, carbohydrate, reference weight, reference height
}

var akg2019 = [...]akgBand{
	{13, 15, [6]float64{2400, 70, 80, 350, 50, 163}, [6]float64{2050, 65, 70, 300, 48, 156}},
	{16, 18, [6]float64{2650, 75, 85, 400, 60, 168}, [6]float64{2100, 65, 70, 300, 52, 159}},
	{19, 29, [6]float64{2650, 65, 75, 430, 60, 168}, [6]float64{2250, 60, 65, 360, 55, 159}},
	{30, 49, [6]float64{2550, 65, 70, 415, 60, 166}, [6]float64{2150, 60, 60, 340, 56, 158}},
	{50, 64, [6]float64{2150, 65, 60, 340, 60, 166}, [6]float64{1800, 60, 50, 280, 56, 158}},
	{65, 80, [6]float64{1800, 64, 50, 275, 58, 164}, [6]float64{1550, 58, 45, 230, 58, 157}},
	{81, 120, [6]float64{1600, 64, 45, 235, 58, 164}, [6]float64{1400, 58, 40, 200, 53, 157}},
}

var akgConditionAdditions = map[string][4]float64{
	"Trimester 1":              {180, 1, 2.3, 25},
	"Trimester 2":              {300, 10, 2.3, 40},
	"Trimester 3":              {300, 30, 2.3, 40},
	"Menyusui 6 bulan pertama": {330, 20, 2.2, 45},
	"Menyusui 6 bulan kedua":   {400, 15, 2.2, 55},
}

func Targets(profile map[string]any) map[string]float64 {
	stored := map[string]float64{
		"calories":  ToFloat(profile["target_calories"]),
		"protein_g": ToFloat(profile["target_protein_g"]),
		"carbs_g":   ToFloat(profile["target_carbs_g"]),
		"fat_g":     ToFloat(profile["target_fat_g"]),
		"sugar_g":   ToFloat(profile["target_sugar_g"]),
		"sodium_mg": ToFloat(profile["target_sodium_mg"]),
	}
	if stored["calories"] > 0 && stored["protein_g"] > 0 && stored["carbs_g"] > 0 && stored["fat_g"] > 0 {
		return stored
	}

	age := ToFloat(profile["age"])
	if age < 13 || age > 120 {
		return targetMap(2100, 57, 260, 70, 2000)
	}
	var values [6]float64
	for _, band := range akg2019 {
		if age >= band.min && age <= band.max {
			if profile["gender"] == "Pria" {
				values = band.male
			} else if profile["gender"] == "Wanita" {
				values = band.female
			}
			break
		}
	}
	if values[0] == 0 {
		return targetMap(2100, 57, 260, 70, 2000)
	}
	weight, height := ToFloat(profile["weight_kg"]), ToFloat(profile["height_cm"])
	gym, _ := profile["is_gym"].(bool)
	activityFactors := map[string]float64{
		"Minim gerak": 1.2, "Ringan": 1.375, "Sedang": 1.55,
		"Sangat aktif": 1.725, "Aktivitas fisik berat": 1.9,
	}
	activity, _ := profile["activity"].(string)
	activityFactor := activityFactors[activity]
	if activityFactor == 0 {
		activityFactor = 1.55
	}
	bodyScale, weightScale := 1.0, 1.0
	if weight > 0 && height > 0 {
		sexOffset := -161.0
		if profile["gender"] == "Pria" {
			sexOffset = 5
		}
		bmr := func(bodyWeight, bodyHeight float64) float64 {
			return 10*bodyWeight + 6.25*bodyHeight - 5*age + sexOffset
		}
		bodyScale = math.Max(.65, math.Min(1.5, bmr(weight, height)/bmr(values[4], values[5])*activityFactor/1.55))
		weightScale = math.Max(.5, math.Min(2, weight/values[4]))
	}
	hasCondition := false
	additions := [4]float64{}
	if profile["gender"] == "Wanita" {
		if status, ok := profile["pregnancy_status"].(string); ok {
			if condition, ok := akgConditionAdditions[status]; ok {
				hasCondition = true
				additions = condition
			}
		}
	}
	calories := values[0]*bodyScale + additions[0]
	if !hasCondition {
		switch profile["goal"] {
		case "Menurunkan berat badan":
			calories -= 300
		case "Menambah berat badan":
			calories += 250
		}
	}
	calories = math.Max(1200, math.Min(5000, calories))
	sodium := 2000.0
	if age <= 15 {
		adultEnergy := 2650.0
		if profile["gender"] == "Wanita" {
			adultEnergy = 2250
		}
		sodium = math.Min(sodium, 2000*calories/adultEnergy)
	}
	protein := math.Round((values[1]*weightScale+additions[1])*10) / 10
	if gym && age >= 19 && !hasCondition {
		protein = math.Round(weight*1.6*10) / 10
	}
	return targetMap(calories,
		protein,
		math.Round((values[3]*weightScale+additions[3])*10)/10,
		math.Round((values[2]*weightScale+additions[2])*10)/10,
		sodium,
	)
}

func targetMap(calories, protein, carbs, fat, sodium float64) map[string]float64 {
	return map[string]float64{
		"calories":  math.Round(calories),
		"protein_g": protein,
		"fat_g":     fat,
		"carbs_g":   carbs,
		"sugar_g":   50,
		"sodium_mg": math.Round(sodium),
	}
}
