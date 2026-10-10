package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNutritionTargetsUsesProfileAndGoal(t *testing.T) {
	targets := nutritionTargets(map[string]any{
		"age": 30, "gender": "Pria", "weight_kg": 70.0, "height_cm": 175.0,
		"activity": "Sedang", "goal": "Menurunkan berat badan",
	})
	if targets["calories"] != 2256 || targets["protein_g"] != 112 || targets["fat_g"] != 63 || targets["carbs_g"] != 311 {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestValidUUID(t *testing.T) {
	if !validUUID("550e8400-e29b-41d4-a716-446655440000") || validUUID("not-a-uuid") {
		t.Fatal("UUID validation failed")
	}
}

func TestNormalizeScanItems(t *testing.T) {
	label := normalizeScanItems(map[string]any{"name": "Susu", "calories": 120.0})
	if len(label) != 1 || label[0]["name"] != "Susu" {
		t.Fatalf("unexpected label items: %#v", label)
	}
	receipt := normalizeScanItems(map[string]any{"items": []any{
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

func TestStripJSONFence(t *testing.T) {
	got := stripJSONFence("```json\n{\"items\": []}\n```")
	if got != "{\"items\": []}" {
		t.Fatalf("unexpected fence strip: %q", got)
	}
	got = stripJSONFence("hasil:\n[{\"name\": \"Roti\"}] selesai")
	if got != "[{\"name\": \"Roti\"}]" {
		t.Fatalf("unexpected array strip: %q", got)
	}
}

func TestGeminiText(t *testing.T) {
	got, err := geminiText([]byte(`{"candidates":[{"content":{"parts":[{"text":"halo "},{"text":"Nutrily"}]}}]}`))
	if err != nil || got != "halo Nutrily" {
		t.Fatalf("geminiText() = %q, %v", got, err)
	}
	if _, err := geminiText([]byte(`{"candidates":[]}`)); err == nil {
		t.Fatal("expected empty candidates to fail")
	}
}

func TestGenerateRetriesServiceUnavailable(t *testing.T) {
	calls := 0
	s := server{
		model:     "test-model",
		geminiKey: "test-key",
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			body := `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`
			status := http.StatusOK
			if calls == 1 {
				status = http.StatusServiceUnavailable
				body = `{"error":{"status":"UNAVAILABLE"}}`
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
		})},
	}
	result, err := s.generate(context.Background(), "", []any{}, false)
	if err != nil || result["answer"] != "ok" || calls != 2 {
		t.Fatalf("generate() = %#v, %v after %d calls", result, err, calls)
	}
}
