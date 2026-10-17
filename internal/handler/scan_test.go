package handler

import (
	"strings"
	"testing"
)

func TestEstimatePromptUsesOnlyUnmatchedItems(t *testing.T) {
	prompt := estimatePrompt([]map[string]any{{"name": "Susu"}}, "label")
	if !strings.Contains(prompt, "Susu") || !strings.Contains(prompt, "Item:") || !strings.Contains(prompt, "perakitan gizi") || !strings.Contains(prompt, "mengarang") || strings.Contains(prompt, "Apify") {
		t.Fatalf("estimatePrompt() omitted items or references old OCR: %q", prompt)
	}
}

func TestMergeGroundedEstimateKeepsOriginalIdentityAndLabelValues(t *testing.T) {
	original := map[string]any{"name": "Susu", "manufacturer": "Merek", "serving_size": "250 ml", "calories": float64(100), "nutrition_source": "label"}
	estimates := []map[string]any{{"name": "Susu bubuk", "serving_size": "1 cup", "calories": float64(120), "protein_g": float64(4), "sources": []map[string]string{{"title": "Nutrition facts", "url": "https://example.com/nutrition"}}}}
	if !mergeGroundedEstimate(estimates, original) {
		t.Fatal("single sourced estimate should pass")
	}
	item := estimates[0]
	if item["name"] != "Susu" || item["manufacturer"] != "Merek" || item["serving_size"] != "250 ml" || item["calories"] != float64(100) || item["protein_g"] != float64(4) || item["fat_g"] != nil || item["nutrition_source"] != "label_estimate" {
		t.Fatalf("grounded merge = %#v", item)
	}
}

func TestMergeGroundedEstimateDoesNotUseUnsourcedValues(t *testing.T) {
	original := map[string]any{"name": "Roti", "nutrition_source": "estimate"}
	estimates := []map[string]any{{"name": "Roti", "calories": float64(120), "protein_g": float64(4)}}
	if !mergeGroundedEstimate(estimates, original) {
		t.Fatal("single estimate should pass")
	}
	if estimates[0]["calories"] != nil || estimates[0]["protein_g"] != nil || len(estimates[0]["sources"].([]map[string]string)) != 0 {
		t.Fatalf("unsourced numbers should be cleared: %#v", estimates[0])
	}
}

func TestMergeGroundedEstimateBatchMapsEachItemByID(t *testing.T) {
	requested := identifyEstimateItems([]map[string]any{{"name": "Bakso"}, {"name": "Susu"}})
	estimates := []map[string]any{
		{"_estimate_id": "1", "name": "Susu", "calories": float64(120), "sources": []map[string]string{{"title": "Sumber susu", "url": "https://example.com/susu"}}},
		{"_estimate_id": "0", "name": "Bakso", "calories": float64(200), "sources": []map[string]string{{"title": "Sumber bakso", "url": "https://example.com/bakso"}}},
	}
	results := mergeGroundedEstimateBatch(estimates, requested)
	if len(results) != 2 || results[0]["name"] != "Bakso" || results[0]["calories"] != float64(200) || results[1]["name"] != "Susu" || results[1]["calories"] != float64(120) {
		t.Fatalf("grounded batch mapping = %#v", results)
	}
}

func TestUnverifiedEstimatesPreserveKnownNutrition(t *testing.T) {
	items := unverifiedEstimates([]map[string]any{{"name": "Susu", "calories": float64(100)}})
	if len(items) != 1 || items[0]["name"] != "Susu" || items[0]["calories"] != float64(100) || items[0]["protein_g"] != nil || len(items[0]["sources"].([]map[string]string)) != 0 {
		t.Fatalf("unverified estimate = %#v", items)
	}
}
