package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCalculateTargetsReturnsDashboardTargetValues(t *testing.T) {
	body := `{"age":30,"gender":"Pria","weight_kg":70,"height_cm":175,"activity":"Sedang","goal":"Menurunkan berat badan","is_gym":false}`
	req := httptest.NewRequest("POST", "/api/targets", strings.NewReader(body))
	response := httptest.NewRecorder()
	(&Handler{}).ServeAPI(response, req)

	if response.Code != 200 {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Targets map[string]float64 `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Targets["calories"] != 2517 || result.Targets["protein_g"] != 75.8 || result.Targets["fat_g"] != 81.7 || result.Targets["carbs_g"] != 484.2 {
		t.Fatalf("preview targets differ from dashboard calculation: %#v", result.Targets)
	}
}

func TestCalculateTargetsUsesGymProteinValue(t *testing.T) {
	body := `{"age":30,"gender":"Pria","weight_kg":70,"height_cm":175,"activity":"Sedang","goal":"Menjaga berat badan","is_gym":true}`
	req := httptest.NewRequest("POST", "/api/targets", strings.NewReader(body))
	response := httptest.NewRecorder()
	(&Handler{}).ServeAPI(response, req)

	var result struct {
		Targets map[string]float64 `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || result.Targets["protein_g"] != 112 {
		t.Fatalf("expected gym target 112 g protein, status %d: %#v", response.Code, result)
	}
}

func TestCalculateTargetsEnforcesMeasurementLimits(t *testing.T) {
	base := `{"age":30,"gender":"Pria","weight_kg":70,"height_cm":175,"activity":"Sedang","goal":"Menjaga berat badan","is_gym":false}`
	for _, test := range []struct {
		name   string
		body   string
		status int
	}{
		{"minimum weight accepted", strings.Replace(base, `"weight_kg":70`, `"weight_kg":20`, 1), 200},
		{"maximum weight accepted", strings.Replace(base, `"weight_kg":70`, `"weight_kg":200`, 1), 200},
		{"minimum height accepted", strings.Replace(base, `"height_cm":175`, `"height_cm":30`, 1), 200},
		{"maximum height accepted", strings.Replace(base, `"height_cm":175`, `"height_cm":200`, 1), 200},
		{"weight below minimum rejected", strings.Replace(base, `"weight_kg":70`, `"weight_kg":19.9`, 1), 400},
		{"weight above maximum rejected", strings.Replace(base, `"weight_kg":70`, `"weight_kg":200.1`, 1), 400},
		{"height below minimum rejected", strings.Replace(base, `"height_cm":175`, `"height_cm":29.9`, 1), 400},
		{"height above maximum rejected", strings.Replace(base, `"height_cm":175`, `"height_cm":200.1`, 1), 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/targets", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			(&Handler{}).ServeAPI(response, req)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}
