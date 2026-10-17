package handler

import (
	"encoding/json"
	"testing"

	"nutrily/backend/internal/nutrition"
)

// TestEstimatePipelineNestedGizi verifies the estimate path flattens nested
// gizi and keeps values. Model-supplied source_urls are intentionally dropped:
// only real web_search citations are trusted.
func TestEstimatePipelineNestedGizi(t *testing.T) {
	fallbackResult := map[string]any{}
	raw := `{"items":[{"_estimate_id":"0","name":"Lemon Tea","serving_size":"1 botol","gizi":{"calories":151,"protein_g":0,"carbs_g":37,"fat_g":0,"sugar_g":36,"sodium_mg":0},"source_urls":["https://fake.example.com/hallucinated"]}]}`
	if err := json.Unmarshal([]byte(raw), &fallbackResult); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Store: &Store{SearchMaxRes: 5}}

	estimates := nutrition.NormalizeScanItems(fallbackResult, "receipt")
	if len(estimates) != 1 {
		t.Fatalf("expected 1 item, got %d", len(estimates))
	}
	if estimates[0]["calories"] != float64(151) {
		t.Fatalf("nested gizi calories not flattened: %#v", estimates[0])
	}

	h.attachOpenAISources(estimates, fallbackResult)

	requested := identifyEstimateItems([]map[string]any{{"name": "Lemon Tea", "serving_size": "1 botol"}})
	merged := mergeGroundedEstimateBatch(estimates, requested)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged item, got %d", len(merged))
	}
	item := merged[0]
	if item["calories"] != float64(151) || item["carbs_g"] != float64(37) {
		t.Fatalf("nutrition lost: %#v", item)
	}
	if sources, _ := item["sources"].([]map[string]string); len(sources) != 0 {
		t.Fatalf("model-supplied source_urls must be discarded, got %#v", sources)
	}
}

// TestEstimatePipelineNestedGiziWithoutSource reproduces the real scan case:
// the model returns nested gizi but an empty source_urls array. With no
// source, mergeGroundedEstimate currently nulls every value.
func TestEstimatePipelineNestedGiziWithoutSource(t *testing.T) {
	fallbackResult := map[string]any{}
	raw := `{"items":[{"_estimate_id":"0","name":"Lemon Tea","gizi":{"calories":8,"protein_g":0.2,"carbs_g":1.5,"fat_g":0,"sugar_g":0.8,"sodium_mg":3},"source_urls":[]}]}`
	if err := json.Unmarshal([]byte(raw), &fallbackResult); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Store: &Store{SearchMaxRes: 5}}
	estimates := nutrition.NormalizeScanItems(fallbackResult, "receipt")
	if len(estimates) != 1 || estimates[0]["calories"] != float64(8) {
		t.Fatalf("nested gizi should flatten even without source: %#v", estimates)
	}
	h.attachOpenAISources(estimates, fallbackResult)
	requested := identifyEstimateItems([]map[string]any{{"name": "Lemon Tea"}})
	merged := mergeGroundedEstimateBatch(estimates, requested)
	t.Logf("merged without source: %#v", merged[0])
	if merged[0]["calories"] != float64(8) {
		t.Fatalf("nutrition was nulled despite valid model values: %#v", merged[0])
	}
}
