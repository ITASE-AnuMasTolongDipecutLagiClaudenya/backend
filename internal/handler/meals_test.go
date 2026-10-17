package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/supabase"
)

func TestCreateMealRejectsInvalidSessionName(t *testing.T) {
	for _, sessionName := range []string{"", "   ", strings.Repeat("é", 61)} {
		t.Run("invalid", func(t *testing.T) {
			body, err := json.Marshal(map[string]string{
				"name":              "Makanan",
				"scan_session_name": sessionName,
			})
			if err != nil {
				t.Fatal(err)
			}

			request := httptest.NewRequest(http.MethodPost, "/api/meals", strings.NewReader(string(body)))
			response := httptest.NewRecorder()
			(&Handler{}).CreateMeal(response, request, middleware.Identity{ID: "user-1"})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("CreateMeal status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestCreateMealRejectsUntrustedNutritionSource(t *testing.T) {
	for _, source := range []string{"http://example.com/nutrition", "https://user:pass@example.com/nutrition", "https://" + strings.Repeat("a", 2048)} {
		body, err := json.Marshal(map[string]any{
			"name":              "Makanan",
			"nutrition_sources": []map[string]string{{"title": "Sumber", "url": source}},
		})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/meals", strings.NewReader(string(body)))
		response := httptest.NewRecorder()
		(&Handler{}).CreateMeal(response, request, middleware.Identity{ID: "user-1"})
		if response.Code != http.StatusBadRequest {
			t.Fatalf("CreateMeal source %q status = %d, want %d", source, response.Code, http.StatusBadRequest)
		}
	}
}

func TestUpdateMealSessionNameUsesUserScopedFilter(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPatch || r.URL.Path != "/rest/v1/meals" || r.URL.Query().Get("user_id") != "eq.user-1" || r.URL.Query().Get("scan_session_id") != "eq.550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("unexpected update request: %s %s", r.Method, r.URL.String())
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["scan_session_name"] != "Scan label 1" {
				t.Errorf("unexpected update body: %v (%v)", body, err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`[{"id":1}]`)), Header: make(http.Header), Request: r}, nil
		})},
	}
	request := httptest.NewRequest(http.MethodPatch, "/api/meals?scan_session_id=550e8400-e29b-41d4-a716-446655440000", strings.NewReader(`{"scan_session_name":"Scan label 1"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	(&Handler{SB: client}).UpdateMealSessionName(response, request, middleware.Identity{ID: "user-1"})
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH meal status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestDeleteMealsRequiresValidTarget(t *testing.T) {
	for _, target := range []string{"", "scan_session_id=invalid", "meal_id=0"} {
		t.Run(target, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodDelete, "/api/meals?"+target, nil)
			response := httptest.NewRecorder()
			(&Handler{}).DeleteMeals(response, request, middleware.Identity{ID: "user-1"})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("DELETE invalid meal target status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestDeleteMealsUsesUserScopedFilter(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodDelete || r.URL.Path != "/rest/v1/meals" || r.URL.Query().Get("user_id") != "eq.user-1" || r.URL.Query().Get("scan_session_id") != "eq.550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("unexpected meal delete request: %s %s", r.Method, r.URL.String())
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`[{"id":1}]`)), Header: make(http.Header), Request: r}, nil
		})},
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/meals?scan_session_id=550e8400-e29b-41d4-a716-446655440000", nil)
	response := httptest.NewRecorder()
	(&Handler{SB: client}).DeleteMeals(response, request, middleware.Identity{ID: "user-1"})
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE meal status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestDeleteMealsReportsNotFoundWhenNoRowsRemoved(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header), Request: r}, nil
		})},
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/meals?scan_session_id=550e8400-e29b-41d4-a716-446655440000", nil)
	response := httptest.NewRecorder()
	(&Handler{SB: client}).DeleteMeals(response, request, middleware.Identity{ID: "user-1"})
	if response.Code != http.StatusNotFound {
		t.Fatalf("DELETE empty meal status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
