package nutrition

import "testing"

func TestTargetsPersonalizeAKGByBodyProfile(t *testing.T) {
	targets := Targets(map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 70.0, "height_cm": 175.0,
		"activity": "Sedang", "goal": "Menurunkan berat badan",
	})
	if targets["calories"] != 2517 || targets["protein_g"] != 75.8 || targets["fat_g"] != 81.7 || targets["carbs_g"] != 484.2 {
		t.Fatalf("unexpected targets: %#v", targets)
	}

	reference := Targets(map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 60.0, "height_cm": 166.0,
		"activity": "Sedang", "goal": "Menurunkan berat badan",
	})
	if reference["calories"] != 2250 || reference["protein_g"] != 65 || reference["fat_g"] != 70 || reference["carbs_g"] != 415 {
		t.Fatalf("unexpected reference targets: %#v", reference)
	}

	lowActivity := Targets(map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 60.0, "height_cm": 166.0,
		"activity": "Minim gerak", "goal": "Menurunkan berat badan",
	})
	if lowActivity["calories"] >= reference["calories"] {
		t.Fatalf("activity should adjust estimated energy: %#v", lowActivity)
	}
}

func TestTargetsUsesOfficialPregnancyAdditionsWithoutWeightLossAdjustment(t *testing.T) {
	targets := Targets(map[string]any{
		"age": 25, "gender": "Wanita", "pregnancy_status": "Trimester 2",
		"goal": "Menurunkan berat badan",
	})
	if targets["calories"] != 2550 || targets["protein_g"] != 70 || targets["fat_g"] != 67.3 || targets["carbs_g"] != 400 {
		t.Fatalf("unexpected pregnancy targets: %#v", targets)
	}
}

func TestTargetsUseDailySugarAndAgeAdjustedSodiumLimits(t *testing.T) {
	adult := Targets(map[string]any{"age": 30, "gender": "Pria"})
	if adult["sugar_g"] != 50 || adult["sodium_mg"] != 2000 {
		t.Fatalf("unexpected adult sugar/sodium targets: %#v", adult)
	}

	teen := Targets(map[string]any{"age": 14, "gender": "Pria"})
	if teen["sugar_g"] != 50 || teen["sodium_mg"] != 1811 {
		t.Fatalf("unexpected teen sugar/sodium targets: %#v", teen)
	}
}

func TestTargetsReadSavedProfileTargets(t *testing.T) {
	profile := map[string]any{
		"age": 30, "gender": "Pria", "target_calories": 2200.0,
		"target_protein_g": 80.0, "target_carbs_g": 250.0,
		"target_fat_g": 70.0, "target_sugar_g": 40.0, "target_sodium_mg": 1800.0,
	}
	targets := Targets(profile)
	if targets["calories"] != 2200 || targets["protein_g"] != 80 || targets["carbs_g"] != 250 || targets["fat_g"] != 70 || targets["sugar_g"] != 40 || targets["sodium_mg"] != 1800 {
		t.Fatalf("expected saved profile targets, got %#v", targets)
	}
}

func TestGymProteinTargetUsesExerciseGuideline(t *testing.T) {
	profile := map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 70.0, "height_cm": 175.0,
		"activity": "Sedang", "goal": "Menjaga berat badan", "is_gym": true,
	}
	if got := Targets(profile)["protein_g"]; got != 112 {
		t.Fatalf("gym protein target = %v, want 112 g (1.6 g/kg)", got)
	}
	profile["is_gym"] = false
	if got := Targets(profile)["protein_g"]; got == 112 {
		t.Fatalf("non-gym protein target unexpectedly uses gym guideline: %v", got)
	}
}

func TestPregnancyProteinAdditionTakesPrecedenceOverGymMultiplier(t *testing.T) {
	profile := map[string]any{
		"age": 30, "gender": "Wanita", "weight_kg": 60.0, "height_cm": 160.0,
		"activity": "Sedang", "goal": "Menjaga berat badan",
		"pregnancy_status": "Trimester 2", "is_gym": true,
	}
	if got := Targets(profile)["protein_g"]; got != 74.3 {
		t.Fatalf("pregnancy protein target = %v, want 74.3 g from AKG", got)
	}
}

func TestNormalizeScanItems(t *testing.T) {
	label := NormalizeScanItems(map[string]any{"name": "Susu", "calories": 120.0}, "label")
	if len(label) != 1 || label[0]["name"] != "Susu" {
		t.Fatalf("unexpected label items: %#v", label)
	}
	receipt := NormalizeScanItems(map[string]any{"items": []any{
		map[string]any{"name": "Roti", "category": "snack", "calories": "200 kcal"},
		"junk",
		map[string]any{"name": "Teh", "category": "beverage", "calories": 90.0},
		map[string]any{"name": "Mi Instan Merek X", "category": "makanan kemasan"},
		map[string]any{"name": "Apel Fuji", "category": "buah-buahan"},
		map[string]any{"name": "Produk XYZ 250ml", "category": "grocery"},
		map[string]any{"name": "Produk ambigu", "category": "grocery", "calories": 0.0},
		map[string]any{"name": "Item tanpa kategori", "calories": 45.0},
		map[string]any{"name": "TOTAL", "calories": 290.0},
	}}, "receipt")
	if len(receipt) != 7 || receipt[0]["name"] != "Roti" || receipt[1]["name"] != "Teh" || receipt[2]["name"] != "Mi Instan Merek X" || receipt[3]["name"] != "Apel Fuji" {
		t.Fatalf("unexpected receipt items: %#v", receipt)
	}
	if receipt[0]["category"] != "makanan" || receipt[1]["category"] != "minuman" || receipt[2]["category"] != "makanan" || receipt[3]["category"] != "buah" || receipt[4]["category"] != "makanan" || receipt[5]["category"] != "makanan" || receipt[6]["category"] != "makanan" {
		t.Fatalf("unexpected categories: %#v", receipt)
	}
	if receipt[0]["calories"] != 200.0 {
		t.Fatalf("expected string calories coerced, got %#v", receipt[0]["calories"])
	}
}

func TestNormalizeScanItemsPreservesGroundingSources(t *testing.T) {
	sources := []map[string]string{{"title": "Nutrition facts", "url": "https://example.com/nutrition"}}
	items := NormalizeScanItems(map[string]any{"items": []any{
		map[string]any{"name": "Susu", "calories": 120.0, "sources": sources},
	}}, "label")
	got, ok := items[0]["sources"].([]map[string]string)
	if len(items) != 1 || !ok || len(got) != 1 || got[0]["url"] != sources[0]["url"] {
		t.Fatalf("grounding sources lost in normalization: %#v", items)
	}
}

func TestNormalizeScanItemsPreservesEstimateID(t *testing.T) {
	items := NormalizeScanItems(map[string]any{"items": []any{
		map[string]any{"_estimate_id": "1", "name": "Bakso", "calories": 200.0},
	}}, "receipt")
	if len(items) != 1 || items[0]["_estimate_id"] != "1" {
		t.Fatalf("estimate ID lost during normalization: %#v", items)
	}
}

func TestNormalizeReceiptInfersCategoryFromProductName(t *testing.T) {
	items := NormalizeScanItems(map[string]any{"items": []any{
		map[string]any{"name": "AQUA 600ML", "category": "grocery"},
		map[string]any{"name": "Pisang Cavendish"},
		map[string]any{"name": "Roti Tawar", "category": "other"},
		map[string]any{"name": "Beras Premium"},
		map[string]any{"name": "Produk XYZ 250ml", "category": "grocery"},
		map[string]any{"name": "Shampoo 250ml"},
	}}, "receipt")
	if len(items) != 5 || items[0]["category"] != "minuman" || items[1]["category"] != "buah" || items[2]["category"] != "makanan" || items[3]["category"] != "makanan" || items[4]["category"] != "makanan" {
		t.Fatalf("receipt categories inferred incorrectly: %#v", items)
	}
}
