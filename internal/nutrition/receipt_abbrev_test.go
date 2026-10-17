package nutrition

import "testing"

func TestNormalizeScanItemsExpandsReceiptAbbreviation(t *testing.T) {
	result := map[string]any{"items": []any{
		map[string]any{
			"name": "KANZLR BAKSO HOT", "product_guess": "Kanzler Bakso Hot",
			"name_confidence": "high", "category": "makanan",
		},
	}}
	items := NormalizeScanItems(result, "receipt")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item["name"] != "Kanzler Bakso Hot" {
		t.Fatalf("name should use the expanded guess: %#v", item["name"])
	}
	if item["raw_name"] != "KANZLR BAKSO HOT" {
		t.Fatalf("raw printed name should be preserved: %#v", item["raw_name"])
	}
	if item["name_confidence"] != "high" {
		t.Fatalf("confidence should be preserved: %#v", item["name_confidence"])
	}
	if _, ok := item["product_guess"]; ok {
		t.Fatal("product_guess should be folded into name and removed")
	}
}

func TestNormalizeScanItemsKeepsNameWithoutGuess(t *testing.T) {
	result := map[string]any{"items": []any{
		map[string]any{"name": "Teh Botol", "category": "minuman"},
	}}
	items := NormalizeScanItems(result, "receipt")
	if len(items) != 1 || items[0]["name"] != "Teh Botol" {
		t.Fatalf("name should stay when no guess: %#v", items)
	}
	if _, ok := items[0]["raw_name"]; ok {
		t.Fatal("raw_name should only be set when an expansion happened")
	}
}

func TestNormalizeScanItemsFlattensNutritionContainer(t *testing.T) {
	result := map[string]any{"items": []any{
		map[string]any{
			"name": "Kanzler Bakso Hot",
			"nutrition": map[string]any{
				"calories": 140, "protein_g": 10, "carbs_g": 5,
				"fat_g": 10, "sugar_g": 0, "sodium_mg": 500,
			},
		},
	}}
	items := NormalizeScanItems(result, "receipt")
	if len(items) != 1 || items[0]["calories"] != float64(140) || items[0]["sodium_mg"] != float64(500) {
		t.Fatalf("nutrition container not flattened: %#v", items)
	}
}

func TestIsNonFoodRejectsHouseholdItems(t *testing.T) {
	for _, name := range []string{"IDM Tusuk Gigi 200's", "IDM Amplop 104A 20's", "Sabun Mandi", "Tisu Wajah"} {
		if !IsNonFood(name) {
			t.Fatalf("%q should be non-food", name)
		}
	}
	for _, name := range []string{"Kanzler Bakso Hot", "Teh Pucuk", "ABC Kacang Hijau"} {
		if IsNonFood(name) {
			t.Fatalf("%q should be food", name)
		}
	}
}
