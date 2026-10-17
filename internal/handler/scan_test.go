package handler

import (
	"testing"

	"nutrily/backend/internal/nutrition"
)

func TestUnverifiedEstimatesPreserveKnownNutrition(t *testing.T) {
	items := unverifiedEstimates([]map[string]any{{"name": "Susu", "calories": float64(100)}})
	if len(items) != 1 || items[0]["name"] != "Susu" || items[0]["calories"] != float64(100) {
		t.Fatalf("unverified estimate = %#v", items)
	}
	if sources, ok := items[0]["sources"].([]map[string]string); !ok || len(sources) != 0 {
		t.Fatalf("unverified estimate must carry no sources: %#v", items[0]["sources"])
	}
}

func TestHasMissingNutrition(t *testing.T) {
	complete := map[string]any{"calories": 1.0, "protein_g": 1.0, "carbs_g": 1.0, "fat_g": 1.0}
	if hasMissingNutrition(complete) {
		t.Fatal("complete item should not report missing nutrition")
	}
	if !hasMissingNutrition(map[string]any{"calories": 1.0}) {
		t.Fatal("partial item should report missing nutrition")
	}
	if !hasAnyNutrition(map[string]any{"sodium_mg": 1.0}) {
		t.Fatal("sodium should count as nutrition")
	}
}

func TestNestedNutritionContainersFlatten(t *testing.T) {
	result := map[string]any{"items": []any{
		map[string]any{
			"name": "Lemon Tea",
			"gizi": map[string]any{"calories": 8, "protein_g": 0.2, "carbs_g": 1.5, "fat_g": 0.0, "sugar_g": 0.8, "sodium_mg": 3},
		},
	}}
	items := nutrition.NormalizeScanItems(result, "receipt")
	if len(items) != 1 || items[0]["calories"] != float64(8) {
		t.Fatalf("nested gizi not flattened: %#v", items)
	}
}
