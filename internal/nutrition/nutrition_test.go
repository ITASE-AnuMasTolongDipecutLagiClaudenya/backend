package nutrition

import "testing"

func TestTargetsUsesProfileAndGoal(t *testing.T) {
	targets := Targets(map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 70.0, "height_cm": 175.0,
		"activity": "Sedang", "goal": "Menurunkan berat badan",
	})
	if targets["calories"] != 2256 || targets["protein_g"] != 112 || targets["fat_g"] != 63 || targets["carbs_g"] != 311 {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestNormalizeScanItems(t *testing.T) {
	label := NormalizeScanItems(map[string]any{"name": "Susu", "calories": 120.0})
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
		map[string]any{"name": "Sabun Mandi", "calories": 0.0},
		map[string]any{"name": "TOTAL", "calories": 290.0},
	}})
	if len(receipt) != 5 || receipt[0]["name"] != "Roti" || receipt[1]["name"] != "Teh" || receipt[2]["name"] != "Mi Instan Merek X" || receipt[3]["name"] != "Apel Fuji" || receipt[4]["name"] != "Produk XYZ 250ml" {
		t.Fatalf("unexpected receipt items: %#v", receipt)
	}
	if receipt[0]["category"] != "makanan" || receipt[1]["category"] != "minuman" || receipt[2]["category"] != "makanan" || receipt[3]["category"] != "buah" {
		t.Fatalf("unexpected categories: %#v", receipt)
	}
	if receipt[0]["calories"] != 200.0 {
		t.Fatalf("expected string calories coerced, got %#v", receipt[0]["calories"])
	}
}
