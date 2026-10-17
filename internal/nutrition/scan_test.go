package nutrition

import (
	"testing"
)

func TestNormalizeNutritionAliasesNestedGizi(t *testing.T) {
	item := map[string]any{
		"name": "Cappuccino Frozen",
		"gizi": map[string]any{
			"calories": 610, "protein_g": 11, "carbs_g": 70,
			"fat_g": 32, "sugar_g": 60, "sodium_mg": 330,
		},
	}
	normalizeNutritionAliases(item)
	if item["calories"] != 610 || item["protein_g"] != 11 || item["carbs_g"] != 70 || item["fat_g"] != 32 || item["sugar_g"] != 60 || item["sodium_mg"] != 330 {
		t.Fatalf("nested gizi not flattened: %#v", item)
	}
	if _, ok := item["gizi"]; ok {
		t.Fatalf("nested gizi key should be deleted: %#v", item)
	}
}
